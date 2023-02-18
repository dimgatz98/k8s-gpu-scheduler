#!/usr/bin/python3


def read_timer(seconds):
    """
    Convert the timers readings into something more legible
    """
    minutes = int(seconds // 60)
    hours = int(minutes // 60)
    seconds = int(seconds % 60)
    return "Elapsed Time: {h} hours, {m} minutes and {s} seconds.".format(
        h=hours, m=minutes, s=seconds
    )
