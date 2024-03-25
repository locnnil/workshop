.. _ref_workshop_remount:

workshop remount
================

Mount a new source location to the content interface plug's target.

.. code-block:: console

   $ workshop remount <WORKSHOP>/<SDK>:<PLUG> <SOURCE> [global options]


Synopsis
--------

This command mounts a new source location on the host to the target directory
of the specified content interface plug, qualified by the SDK name.
Specifically, it does the following:

- Attempts the mount operation atomically;
  this normally succeeds if the new source is either a non-existing directory
  or an empty directory on the same file system as the current source
- Otherwise, performs the mount operation only if the workshop is *Stopped*
  to prevent data corruption


Notes
-----

- To stop the workshop, use :ref:`ref_workshop_stop`
- :ref:`ref_workshop_info` explicitly lists any remounted plugs for a workshop
- :ref:`ref_workshop_refresh` mounts the last source
  set by :command:`workshop remount`, if any
- During :ref:`ref_workshop_remove`,
  non-default sources set by :command:`workshop remount` aren't removed


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
- :ref:`exp_sdk`


Reference:

- :ref:`ref_workshop_info`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_remove`
- :ref:`ref_workshop_stop`
