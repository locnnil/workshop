.. _exp_mount_interface:

Mount interface
===============

The mount interface
exposes host file system locations
to individual SDKs
by mounting them inside the workshop
that references those SDKs.

By using the mount interface,
the SDK publisher allows to persist data outside the workshop.
The interface defines a target directory inside the workshop,
to which a source directory from the host file system is mounted at run-time.
Typically, this is a directory that stores SDK-specific data,
accumulated over time or created
when the :command:`workshop launch` or :command:`workshop refresh` commands run.


Connection
----------

The interface is connected automatically at launch or refresh,
provided that the plug can be matched to the slot by its name
or via a :samp:`connections` entry in the :ref:`definition <exp_workshop_def>`,
both subject to |project_markup|'s
:ref:`validation rules <exp_interfaces_validation>`.

Establishing a connection means
a directory created by |project_markup| on the host file system
is mounted to the target directory inside the workshop;
the best part is that it's preserved
between |project_markup| operations such as
:command:`workshop refresh`, :command:`workshop start`
and :command:`workshop stop`,
so you benefit from a pre-populated directory without doing extra work.

To check if the interface is connected:

.. code-block:: console

   $ workshop connections --all

     Interface  Plug                   Slot        Notes
     ...
     ssh-agent  ws/ssh-sdk:ssh-agent   :ssh-agent  manual


So the target directory is available on the host:

.. code-block:: console

   $ workshop info ws

     name:     ws
     base:     ubuntu@22.04
     project:  /home/user/workshops/ws
     status:   ready
     notes:    -
     content:
       content-sdk:
         channel:  latest/edge
         mounts:
           content-cache:
             host-source:      .../8584e571/mount/ws_content-sdk_content-cache.sdk
             workshop-target:  /home/workshop/target


By default, the source directory on the host
is created by |project_markup| in a designated internal location;
this is done for security reasons.


Remount
-------

The :command:`workshop remount` command sets a new source directory on the host
for the target directory inside the workshop:

.. code-block:: console

   $ workshop remount ws/content-sdk:content-share ~/.local/share/


First, the remount operation is attempted atomically;
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

- :ref:`ref_workshop_connect`
- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_disconnect`
- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_remount`
- :ref:`ref_workshop_start`
- :ref:`ref_workshop_stop`
