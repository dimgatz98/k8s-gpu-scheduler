#!/usr/bin/python3

import re
import subprocess

p = subprocess.Popen("nvidia-smi -L", stdout=subprocess.PIPE, shell=True)

nvidia_out = p.communicate()[0].decode('utf-8').split('\n')
uuids = []
for out in nvidia_out:
	uuid4hex = re.compile('[a-f0-9]{8}-?[a-f0-9]{4}-?[a-f0-9]{4}-?[a-f0-9]{4}-?[a-f0-9]{12}', re.I)
	if uuid4hex.search(out):
		uuids.append(
            f'MIG-{uuid4hex.search(out).group(0)}' if 'MIG' in out 
            else f'GPU-{uuid4hex.search(out).group(0)}'
        )

print(uuids)
