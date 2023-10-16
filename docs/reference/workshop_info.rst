.. _ref_workshop_info:

workshop info
==============

Prints the current status and details of a workshop as YAML.

.. code:: shell

   workshop info <WORKSHOP> [global options]


Synopsis
--------

This command outputs the basic settings, current status and individual SDK
details for a workshop, formatting them as YAML.  Specifically, it prints:

- Essential workshop attributes, such as name, base and project directory
- Current status (e.g. *Ready*, *Pending*, *Off*) and notes for the workshop
- Individual SDK details, such as name, channel, installation date and revision


Notes
-----

- Avoid assumptions based on SDK channels: ``latest/stable`` may be neither


Global options
--------------

-h, --help

  Print the help message for the command.

-p, --project <DIRECTORY>

  Specify the project's directory path.


See also
--------

Explanation:

- :ref:`SDK (concept) <exp_sdk>`
- :ref:`workshop base (concept) <exp_workshop_base>`
- :ref:`workshop definition (concept) <exp_workshop_def>`
