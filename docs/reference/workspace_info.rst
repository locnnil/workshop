.. _workspace_info:

workspace info
==============

Prints the current status and details of a workspace as YAML.

.. code:: shell

   workspace info <WORKSPACE> [global options]


Synopsis
--------

This command outputs the basic settings, current status and individual SDK
details for a workspace, formatting them as YAML.  Specifically, it prints:

- Essential workspace attributes, such as name, base and project directory
- Current status (e.g. *Ready*, *Pending*, *Off*) and notes for the workspace
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
- :ref:`workspace base (concept) <exp_workspace_base>`
- :ref:`workspace definition (concept) <exp_workspace_def>`
