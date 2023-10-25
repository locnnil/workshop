.. _ref_workshop_remove:

workshop remove
===============

Removes one or many workshops.

.. code:: shell

   workshop remove <WORKSHOP>... [global options]


Synopsis
--------

This command removes the workshops listed as arguments.
For each workshop, it:

- Checks that the workshop isn't *Off* or *Pending*
- Stops the workshop if it's not already *Stopped*
- Deletes the workshop but preserves its definition


Notes
-----

- If any listed workshop is *Off* or *Pending*, none are removed
- To rebuild a removed workshop from scratch, use :ref:`ref_workshop_launch`


Global options
--------------

-h, --help

  Print the help message for the command.

-p, --project <DIRECTORY>

  Specify the project's directory path.


See also
--------

Explanation:

- :ref:`project (concept) <exp_project>`
- :ref:`workshop (concept) <exp_workshop>`
- :ref:`workshop definition (concept) <exp_workshop_def>`

Reference:

- :ref:`workshop launch (command) <ref_workshop_launch>`
- :ref:`workshop stop (command) <ref_workshop_stop>`
