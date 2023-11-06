.. _ref_workshop_launch:

workshop launch
===============

Constructs one or many workshops using their definitions.

.. code:: shell

   workshop launch <WORKSHOP>... [global options]


Synopsis
--------

This command constructs the workshops listed as arguments by going over their
definitions and installing their components.  For each workshop, it:

- Checks the workshop definition and identifies necessary actions
- Retrieves the required components, such as base and SDKs
- Runs SDK setup hooks to initialise the working state
- On success, ties the workshop to the project and starts it

If multiple workshops are listed and an error occurs,
the operation is aborted and no workshops are constructed.


Notes
-----

- Names listed as arguments must match
  respective :code:`name:` values in definitions
- To update an existing workshop, use :ref:`ref_workshop_refresh` instead
- SDKs are installed in alphabetical order



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
- :ref:`project (concept) <exp_project>`
- :ref:`workshop base (concept) <exp_workshop_base>`
- :ref:`workshop definition (concept) <exp_workshop_def>`

Reference:

- :ref:`workshop refresh (command) <ref_workshop_refresh>`
- :ref:`workshop remove (command) <ref_workshop_remove>`
- :ref:`workshop start (command) <ref_workshop_start>`
