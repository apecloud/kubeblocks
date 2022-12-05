#!/usr/bin/env python
# -*- coding:utf-8 -*-
import argparse
import re
import os

parser = argparse.ArgumentParser(description='pcregrep checks Chinese characters')
parser.add_argument('--source', '-s', help='pcregrep source file path', default="pcregrep.out")
parser.add_argument('--filter', '-f', help='push incremental file path', default="")

args = parser.parse_args()
source_file_path = args.source
filter_file_path = args.filter


# Check only incremental files
def pcregrep_Chinese(file_path, filter_path):
    check_pass = True
    # Check for incremental files
    if not filter_path:
        print('incremental files not found!')
        return None

    # Check for pcregrep file
    if not os.path.isfile(file_path):
        print(file_path + ' not found!')
        return None

    filter_paths = filter_path.splitlines()
    pattern = '[\u4e00-\u9fa5]'
    pat = re.compile(pattern)

    with open(file_path, 'rb') as doc:
        lines = doc.readlines()
        for line in lines:
            try:
                line = line.decode(encoding="utf-8")
                # There are Chinese characters
                if pat.findall(line):
                    for path in filter_paths:
                        # Matching incremental files
                        if path and path + ":" in line:
                            print(line[:-1])
                            check_pass = False
                            break
            except UnicodeDecodeError:
                # Ignore coding error messages
                continue

    if check_pass:
        print('pcregrep check success!')
    else:
        raise Exception("The submitted files contains Chinese characters!")


if __name__ == '__main__':

    pcregrep_Chinese(source_file_path, filter_file_path)
