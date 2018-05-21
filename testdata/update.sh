#!/bin/sh -e
# update the contents of dir.cpio to reflect dir/
find dir/ -type f | cpio -H newc -o > dir.cpio
