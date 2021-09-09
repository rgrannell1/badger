#!/usr/bin/python3

"""Usage:
    badger copy --from <srcdir> --to <destdir> [-s <num>|--max-seconds-diff <num>] [-b <threshold>|--blur-threshhold <threshold>]
    badger flatten --from <srcdir> --to <destdir>
    badger (-h|--help)

Description:
    badger is a tool that helps you filter large folders of photos. It:

    - Copies media into a directory, with photos / video clustered together by the time they were taken
    - Updates photo names with an estimate of how blurry they are, so you can sort from likey sharpest to blurriest within a burst of photos.
    - Flattens the final selection of media into a folder ready to upload.

Clustering Technique:
    badger clusters images using DBSCAN, which is quite effective at finding groups of media taken at a similar time and excluding "noise" - photos or videos taken
    at a much different time than other media. It can be controlled using --max-seconds-diff (described below).

Blur Detection:
    badger uses the variance of the image's grayscale Laplacian to estimate blur (https://www.pyimagesearch.com/2015/09/07/blur-detection-with-opencv/). It is imperfect,
    but it is a useful indicator of image-quality and is a useful method of sorting a particular scene from blurriest to least-blurry. Use --blur-threshold to remove
    very blurry images

Options:
    --from <srcdir>                                  the source directory, often an SD card's /media folder.
    --to <destdir>                                   the destination directory; please DO NOT attempt to write to the source-directory, you will almost certainly lose your media!
    -s <num>, --max-seconds-diff <num>               two photos / videos this number of seconds apart or less will be clustered into the same folder. Raise to group more media, lower to
                                                         reduce the number of grouped media.
    -b <threshold>, --blur-threshhold <threshold>    should we ignore blurry images, as measured by the variance of the image laplacian? By default all images are copied.
                                                         50 would be a conservative threshold, 200 would be aggressive. Disabled by default, all pictures are copied.
    -h, --help                                       show this documentation.
"""

import multiprocessing
import badger

import signal
import os
import shutil
import glob
from typing import Any
import numpy as np
import cv2

from docopt import docopt
from sklearn.cluster import DBSCAN
import pathlib
from datetime import datetime
from PIL import Image, ExifTags
from alive_progress import alive_bar

from multiprocessing.pool import Pool

import logging
logging.basicConfig(level=logging.INFO, format='🦡 %(message)s')

CLUSTER_SIZE = 2
SUPPORTED_BLUR_EXTENSIONS = ['png', 'jpg', 'jpeg']


def list_media_by_date(fpath: str):
    """List files matching a glob pattern"""
    matches = glob.glob(fpath)

    for fpath in matches:
        # -- use EXIF data if possible
        try:
            img = Image.open(fpath)
            exif_data = img.getexif()

            for key, val in exif_data.items():
                if key in ExifTags.TAGS:
                    tag = ExifTags.TAGS[key]

                    # -- are there other tags that might be useful?
                    if tag == 'DateTime':
                        seconds = datetime.strptime(
                            val, '%Y:%m:%d %H:%M:%S').strftime('%s')
                        yield {
                            'seconds': float(seconds),
                            'fpath': fpath
                        }
        except:
            # -- fallback to ctime
            stat = pathlib.Path(fpath).stat()

            yield {
                'seconds': float(stat.st_ctime),
                'fpath': fpath
            }


def yes_or_no(question: str) -> bool:
    while "the answer is invalid":
        reply = str(input(question+' (y/n): ')).lower().strip()
        if reply[:1] == 'y':
            return True
        if reply[:1] == 'n':
            return False
    return False


class CTError(Exception):
    def __init__(self, errors):
        self.errors = errors


try:
    O_BINARY = os.O_BINARY
except:
    O_BINARY = 0
READ_FLAGS = os.O_RDONLY | O_BINARY
WRITE_FLAGS = os.O_WRONLY | os.O_CREAT | os.O_TRUNC | O_BINARY
BUFFER_SIZE = 128*1024


def copyfile(src, dst):
    with open(src, 'rb') as fda:
        with open(dst, 'wb') as fdb:
                shutil.copyfileobj(fda, fdb, length=64*1024)


def copy_utility(src: str, dest: str):
    # we might not be copying over the best connection, let's compress-and-copy
    # and push the work to the cpu instead!
    copyfile(src, dest)

    #with open(src, mode='rb') as conn:
    #    with lz4.frame.open(dest + '.lz4', mode='wb') as fp:
    #        fp.write(conn.read())


def copy_file(entry):
    x, tgt, media_id = entry
    data, id = x

    _, file_extension = os.path.splitext(data['fpath'])
    file_extension = file_extension.lower()

    new_filename = f'{media_id}{file_extension}'

    if id == -1:
        # -- a noise point, outside of a cluster. Just copy this to the destination folder directly
        to = os.path.join(tgt, new_filename)

        copy_utility(data['fpath'], to)
        return to

    else:
        # -- a cluster point. copy to a cluster folder inside the destination folder
        tgt_dir = os.path.join(tgt, str(id))

        try:
            # -- construct the cluster subdirectory
            os.makedirs(tgt_dir)
        except FileExistsError:
            pass
        except:
            raise

        # -- move the file to an initial copy location
        to = os.path.join(tgt_dir, new_filename)

        # -- lets skip existing files for speed, we might want a force option later
        if not pathlib.Path(to).exists():
            copy_utility(data['fpath'], to)

        return to


# https://www.pyimagesearch.com/2015/09/07/blur-detection-with-opencv/

def rename_by_image_blur(fpath: str):
    """Rename supported images to indicate how blurry they are"""
    if not any([fpath.lower().endswith(ext) for ext in SUPPORTED_BLUR_EXTENSIONS]):
        return

    try:
        # -- estimate blur using an image laplacian
        image = cv2.imread(fpath)
        gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)

        blur = cv2.Laplacian(gray, cv2.CV_64F).var()

        # -- assemble the new filename
        dirname = os.path.dirname(fpath)
        base = os.path.basename(fpath)
        name, ext = os.path.splitext(base)

        prefix = str(round(blur, 1)).replace('.', '')

        tgt = os.path.join(dirname, f'{prefix}_{name}{ext}')

        # -- apply the file renaming
        os.rename(fpath, tgt)

    except Exception as err:
        print(f'failed to rename {fpath} according to blur: {err}')
        pass


def initialiser():
    """Ignore CTRL+C in the worker process."""
    signal.signal(signal.SIGINT, signal.SIG_IGN)

def copy_media_files(dpath: str, files_by_date, clustering) -> None:
    """Copy media files to a new directory, grouped by cluster"""

    media_id = 0
    total_file_count = len(files_by_date)

    entries = zip(list(files_by_date), clustering.labels_)

    nproc = max(multiprocessing.cpu_count() - 1, 1)

    logging.info(
        f'spinning up {nproc} processes to copy & analyse files.\n')

    maps = [[entry, dpath, idx] for idx, entry in enumerate(entries)]

    # -- this copies reasonably fast on my machine;
    # -- IO % is maxed out, disk-writes of about 70 M/s
    with Pool(nproc, initializer=initialiser) as pool:
        try:
            with alive_bar(len(files_by_date)) as bar0:
                dests = []

                for copied in pool.imap_unordered(copy_file, maps):
                    dests.append(copied)
                    bar0()

            with alive_bar(len(dests)) as bar1:
                for _ in pool.imap_unordered(rename_by_image_blur, dests):
                    bar1()
        except KeyboardInterrupt:
            # -- we suppressed sigints (ctrl + c), we need explicit handling here
            pool.terminate()
            pool.join()


def copySubcommand(args: dict[str, Any]):
    logging.info(
        f'reading media creation-times from {args["--from"]}, be patient...')

    files_by_date = list(list_media_by_date(args['--from']))

    if len(files_by_date) == 0:
        logging.error(
            f'did not find any files in {args["--from"]}; is your device connected and mounted?')
        exit(1)

    # -- cluster files by date
    sec_diff = int(args['--max-seconds-diff'])

    seconds = np.array([data['seconds']
                       for data in files_by_date]).reshape(-1, 1)
    clustering = DBSCAN(eps=sec_diff, min_samples=CLUSTER_SIZE).fit(seconds)
    max_count = max(clustering.labels_)

    # -- prompt whether the user wants to proceed copying from one folder to the other
    srcdir = args['--from']
    destdir = args['--to']
    answer = yes_or_no(
        f'> would you like to copy {len(files_by_date)} files from {srcdir} into {max_count} cluster-folders in {destdir}?')

    if not answer:
        return

    copy_media_files(destdir, files_by_date, clustering)


def flattenSubcommand(args: dict[str, Any]):
    pass


def main():
    """Call the correct CLI command"""

    args = docopt(__doc__, version='Badger 0.1')

    if args['--from'] == args['--to']:
        logging.error('--to and --from arguments cannot be the same')
        exit(1)

    if args['copy']:
        copySubcommand(args)
    elif args['flatten']:
        flattenSubcommand(args)


if __name__ == '__main__':
    try:
      main()
    except KeyboardInterrupt:
        print('\nExiting...')
        exit(1)
