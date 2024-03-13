.. _ref_workshop_remove:

workshop remove
===============

Removes one or many workshops.

.. code-block:: console

   $ workshop remove <WORKSHOP>... [global options]


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

- :ref:`exp_project`
- :ref:`exp_workshop`
- :ref:`exp_workshop_def`

Reference:

- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_stop`
