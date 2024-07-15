.. _ref_workshop_remove:

workshop remove
===============

Removes one or many workshops.

.. code-block:: console

   $ workshop remove <WORKSHOP>... [OPTIONS]


Examples
--------

Remove the :samp:`nimble` and :samp:`jazzy` workshops
in the current project directory:

.. code-block:: console

   $ workshop remove nimble jazzy


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

- For content interface plugs,
  non-default sources set by :ref:`ref_workshop_remount` aren't removed


Global options
--------------

-h, --help

  Print the help message for the command.

-p, --project <DIRECTORY>

  Specify the project's directory path.


See also
--------

Explanation:

- :ref:`exp_content_interface`
- :ref:`exp_projects`
- :ref:`exp_workshop`
- :ref:`exp_workshop_def`

Reference:

- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_remount`
- :ref:`ref_workshop_stop`
