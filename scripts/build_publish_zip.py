# -*- coding: utf-8 -*-
# Standard zip packer used by build-publish.ps1.
# Usage: python build_publish_zip.py <src_dir> <out_zip>
import os
import sys
import zipfile

src, out = sys.argv[1], sys.argv[2]
if os.path.exists(out):
    os.remove(out)

count = 0
with zipfile.ZipFile(out, 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as z:
    for root, dirs, files in os.walk(src):
        dirs.sort()
        for f in sorted(files):
            p = os.path.join(root, f)
            arc = os.path.relpath(p, src).replace(os.sep, '/')
            z.write(p, arc)
            count += 1

print('entries=%d size=%d' % (count, os.path.getsize(out)))
