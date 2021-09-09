#!/usr/bin/python3

"""Usage:
    badger copy --from <srcdir> --to <destdir> [-s <num>|--max-seconds-diff <num>] [-b <threshold>|--blur-threshhold <threshold>]
    badger flatten --from <srcdir> --to <destdir>
    badger [-h|--help]

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

import logging
logging.basicConfig(level=logging.INFO, format='📷 %(message)s')

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


def copy_media_files(args, files_by_date, clustering) -> None:
    """Copy media files to a new directory, grouped by cluster"""

    media_id = 0
    total_file_count = len(files_by_date)

    for data, id in zip(files_by_date, clustering.labels_):
        _, file_extension = os.path.splitext(data['fpath'])
        file_extension = file_extension.lower()

        new_filename = f'{media_id}{file_extension}'
        media_id += 1

        if id == -1:
            # -- a noise point, outside of a cluster. Just copy this to the destination folder directly
            to = os.path.join(args['--to'], new_filename)

            logging.info(
                f'copying {str(media_id)} / {str(total_file_count)} {data["fpath"]} to {to}')

            shutil.copy(data['fpath'], to)

        else:
            # -- a cluster point. copy to a cluster folder inside the destination folder
            tgt_dir = os.path.join(args['--to'], str(id))

            try:
                # -- construct the cluster subdirectory
                os.makedirs(tgt_dir)
            except FileExistsError:
                pass
            except:
                raise

            # -- move the file to an initial copy location
            to = os.path.join(tgt_dir, new_filename)
            logging.info(
                f'copying {str(media_id)} / {str(total_file_count)} {data["fpath"]} to {to}')

            shutil.copy(data['fpath'], to)

            # -- rename by blur, if possible
            rename_by_image_blur(to)

    logging.info(f'done')


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
        shutil.move(fpath, tgt)

    except Exception as err:
        print(f'failed to rename {fpath} according to blur: {err}')
        pass


def copySubcommand(args: dict[str, Any]):
    logging.info(
        f'reading media creation times from {args["--from"]}, be patient...')

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
    answer = yes_or_no(
        f'would you like to copy {len(files_by_date)} files into {max_count} clusters in {args["--from"]} to entries within {args["--to"]}?')

    if not answer:
        return

    copy_media_files(args, files_by_date, clustering)


def flattenSubcommand(args: dict[str, Any]):
    pass


def main():
    """Call the correct CLI command"""

    args = docopt(__doc__, version='Badger 0.1')

    if not args['copy'] and not args['flatten']:


    if args['--from'] == args['--to']:
        logging.error('--to and --from arguments cannot be the same')
        exit(1)

    if args['copy']:
        copySubcommand(args)
    elif args['flatten']:
        flattenSubcommand(args)


if __name__ == '__main__':
    main()
