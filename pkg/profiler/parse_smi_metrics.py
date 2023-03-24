#!/usr/bin/python3

import pandas as pd
import time
import subprocess

import signal
from datetime import datetime
import os

run = True


def cls():
    os.system("cls" if os.name == "nt" else "clear")


def handler(signum, stack_frame):
    global run
    run = False


if __name__ == "__main__":
    signal.signal(signal.SIGINT, handler)
    df = pd.DataFrame([], columns=["Power", "Util", "Temp"], index=[])
    while run:
        p = subprocess.Popen(
            "nvidia-smi --format=csv --query-gpu=power.draw,utilization.gpu,temperature.gpu",
            stdout=subprocess.PIPE,
            shell=True,
        )
        s = p.communicate()[0].decode()
        l = s.split("\n")[1].split(",")
        l = [str(0) if "N/A" in x else x.strip("W").strip("%").strip() for x in l]

        current_date_and_time = datetime.now()
        df.loc[current_date_and_time] = l
        cls()
        print(df)
        time.sleep(1)

    df.to_csv("test.ods", index=True, header=True, mode="w", sep="\t")
