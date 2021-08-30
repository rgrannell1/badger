
"""Usage:
group-photos-by-date copy --from <srcdir> --to <destdir> [--min-seconds-diff <num>]

Description:
  Copy folders, videos, and arbitrary media into a directory, and cluster photos taken at roughly the same time into a folders. This is useful to group multiple images of the same subject, and then choose the best of the set.

  After you have tidied up unneeded duplicates in each cluster-folder, flatten media into a single directory.

Options:
  --from <srcdir>
  --to <destdir>
  --min-seconds-diff <num>
"""

import os
import shutil
import glob
import numpy as np
from sklearn.cluster import DBSCAN
import pathlib
from datetime import datetime
from docopt import docopt
from PIL import Image, ExifTags

import logging
logging.basicConfig(level=logging.INFO, format='📷 %(message)s')


def list_by_date(fpath: str):
    """List files matching a glob pattern"""
    matches = glob.glob(fpath)

    for fpath in matches:
        # -- use exif if possible
        try:
            img = Image.open(fpath)
            exif_data = img.getexif()

            for key, val in exif_data.items():
                if key in ExifTags.TAGS:
                    tag = ExifTags.TAGS[key]

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


def yes_or_no(question):
    while "the answer is invalid":
        reply = str(input(question+' (y/n): ')).lower().strip()
        if reply[:1] == 'y':
            return True
        if reply[:1] == 'n':
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
                os.makedirs(tgt_dir)
            except FileExistsError:
                pass
            except:
                raise

            to = os.path.join(tgt_dir, new_filename)
            logging.info(
                f'copying {str(media_id)} / {str(total_file_count)} {data["fpath"]} to {to}')


            shutil.copy(data['fpath'], to)

    logging.info(f'done')


CLUSTER_SIZE = 2


def main():
    """Call the correct CLI command"""

    args = docopt(__doc__, version='Box 1.0')

    if args['--from'] == args['--to']:
        logging.error('--to and --from arguments cannot be the same')
        exit(1)

    logging.info(
        f'reading media creation times from {args["--from"]}, be patient...')

    files_by_date = list(list_by_date(args['--from']))

    if len(files_by_date) == 0:
        logging.error(
            f'did not find any files in {args["--from"]}; is your device connected?')
        exit(1)

    sec_diff = int(args['--min-seconds-diff'])

    seconds = np.array([data['seconds']
                       for data in files_by_date]).reshape(-1, 1)
    clustering = DBSCAN(eps=sec_diff, min_samples=CLUSTER_SIZE).fit(seconds)

    max_count = max(clustering.labels_)
    answer = yes_or_no(
        f'would you like to copy {len(files_by_date)} files into {max_count} clusters in {args["--from"]} to entries within {args["--to"]}?')

    if not answer:
        return

    copy_media_files(args, files_by_date, clustering)


if __name__ == '__main__':
    main()
