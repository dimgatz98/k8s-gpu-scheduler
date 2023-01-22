#!/usr/bin/python3

input()
data = input()
data = data.split(',')
data = [x.strip('%').strip() for x in data]
for d in data:
    if 'N/A' not in d:
        print(d)
    else:
        print(0)
