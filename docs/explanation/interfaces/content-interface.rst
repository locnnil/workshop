.. _exp_content_interface:

Content interface
=================

The content interface
exposes host file system locations
to individual SDKs
by mounting them inside the workshop
that references those SDKs.


Content interface plug
----------------------

An essential element here is the content interface plug,
which is declared in the :ref:`SDK definition <exp_sdk_definition>`
and is thus beyond the reach of |project_markup|.
The plug defines a target directory inside the workshop,
to which the source directory from the slot is mounted at run-time.

Typically, this is a directory that stores SDK-specific data,
accumulated over time or created at :command:`launch` or :command:`refresh`;
by adding a plug,
the SDK publisher allows the target data to persist outside the workshop.


Content interface slot
----------------------

To enable this mechanism,
|project_markup| provides a content interface slot
to which multiple content interface plugs can
:ref:`connect <exp_interface_connections>`.

.. note::

   Currently, content can only be exposed by |project_markup| itself
   and can't be shared directly between two workshops.


When an SDK is installed
during :command:`launch` and :command:`refresh`,
|project_markup| checks that the plug targeting the slot
passes :ref:`validation <exp_interfaces_validation>`
and that the :samp:`target` directory already exists in the workshop.
If the plug passes these checks,
it is automatically connected.

To ensure the plug is connected to the slot:

.. code-block:: console

   $ workshop connections --all

     Interface  Plug                   Slot        Notes
     ...
     ssh-agent  ws/ssh-sdk:ssh-agent   :ssh-agent  manual


This means a |project_markup|-created directory in the host file system
is mounted to the :samp:`target` directory inside the workshop.
This source directory is retained between workshop operations such as
:command:`refresh`, :command:`start` and :command:`stop`,
so you can benefit from a pre-populated target without having to redo the work.


Remounting plugs
----------------

The :command:`remount` command sets a new source directory on the host
for the plug's :samp:`target` inside the workshop:

.. code-block:: console

   $ workshop remount ws/content-sdk:content-share ~/.local/share/


First, the mount operation is attempted atomically;
this usually succeeds if the new source is either a non-existent directory
or an empty directory on the same file system as the current source.
Otherwise, the remount only occurs if the workshop has been stopped earlier,
which prevents data corruption.


See also
--------

Explanation:

- :ref:`exp_interfaces`
- :ref:`exp_plugs_slots`
- :ref:`exp_sdk_definition`
- :ref:`exp_workshop_def`


Reference:

- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_remount`
- :ref:`ref_workshop_start`
- :ref:`ref_workshop_stop`
