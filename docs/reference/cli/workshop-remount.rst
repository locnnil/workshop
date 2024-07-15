.. _ref_workshop_remount:

workshop remount
================

Mount a new source location to the content interface plug's target.

.. code-block:: console

   $ workshop remount <WORKSHOP>/<SDK>:<PLUG> <SOURCE> [OPTIONS]


Examples
--------

Remount the :samp:`mod-cache` content interface plug
of the :samp:`go` SDK under the :samp:`nimble` workshop
in the current project directory
to :file:`~/new-cache-mount/` on the host:

.. code-block:: console

   $ workshop remount nimble/go:mod-cache ~/new-cache-mount/


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

- :ref:`ref_workshop_info` lists any mounted content interface plugs
  for the workshop

- :ref:`ref_workshop_refresh` mounts the last source
  set by :command:`workshop remount`, if any

- During :ref:`ref_workshop_remove`,
  non-default sources set by :command:`workshop remount` aren't removed


Options
-------

--no-wait

  Return the change ID, don't wait for the operation to finish.


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

- :ref:`ref_workshop_connect`
- :ref:`ref_workshop_disconnect`
- :ref:`ref_workshop_info`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_remove`
- :ref:`ref_workshop_stop`
