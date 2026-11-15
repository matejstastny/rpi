"""the fleet cli. see cli/main.py for the command surface."""


class Fault(Exception):
    """a problem the person running pi can fix.

    these are printed as-is, with no exception type in front, because the
    message is the whole point. anything else that escapes is a bug and keeps
    its type so it is obvious which.
    """
