.. _how_use_host_devices:

.. meta::
   :description: How-to guide on accessing host devices inside a workshop with
                 the custom device interface, covering subsystem discovery,
                 declaring a plug in an SDK, connecting the interface manually,
                 and reaching the devices inside the workshop.

How to use host devices
=======================

.. @tests in tests/docs-how-to/use-host-devices/task.yaml

.. @artefact custom-device interface

|ws_markup| exposes arbitrary host devices to a workshop
through the custom device interface,
identified by the device *subsystem* they belong to
(for example, :samp:`input`, :samp:`tty`, or :samp:`usb`).
A plug declared on an SDK names the subsystem;
|ws_markup| then passes every host device in that subsystem
into the workshop once the plug is connected.

The interface is never connected automatically,
so reaching a host device follows a short sequence:
find the subsystem, declare a plug, connect the interface,
then use the devices inside the workshop.


Identify the device subsystem
-----------------------------

.. @artefact custom-device interface attributes

Every device node on the host belongs to a subsystem
defined by the Linux kernel.
Query the subsystem of a device with :command:`udevadm info`:

.. code-block:: console

   $ udevadm info --query=property --property=SUBSYSTEM /dev/input/event0

     SUBSYSTEM=input


The device at :file:`/dev/input/event0` belongs to the :samp:`input` subsystem.
That name is what the plug declares in the next step.


Declare a custom device plug
----------------------------

A custom device plug lives on an SDK, not on the workshop directly.
To add one without publishing a separate SDK,
declare an in-project SDK:
a small definition stored under :file:`.workshop/`
that the workshop references with the :samp:`project-` prefix.

Give the SDK a name and declare the plug,
naming the subsystem from the previous step:

.. code-block:: yaml
   :caption: .workshop/input-sdk/sdk.yaml

   name: input-sdk
   plugs:
     input-device:
       interface: custom-device
       subsystem: input


Then reference the in-project SDK from the workshop definition:

.. code-block:: yaml
   :caption: .workshop/dev.yaml
   :emphasize-lines: 4

   name: dev
   base: ubuntu@24.04
   sdks:
     - name: project-input-sdk


.. note::

   The :samp:`project-` prefix appears only in the :samp:`sdks:` list.
   |ws_markup| strips it internally,
   so the SDK keeps its bare name, :samp:`input-sdk`,
   in connections and command arguments.


Connect the interface
---------------------

.. @artefact workshop connect

Launch the workshop to install the SDK:

.. code-block:: console

   $ workshop launch dev


The custom device interface stays disconnected after launch,
for security reasons.
Connect the plug to the workshop's custom device slot by hand:

.. code-block:: console

   $ workshop connect dev/input-sdk:input-device :custom-device


The first argument is the plug, :samp:`<WORKSHOP>/<SDK>:<PLUG>`.
The trailing :samp:`:custom-device` selects the slot
that the built-in system SDK provides for every workshop.

Connecting the plug makes all existing host devices in the subsystem
available inside the workshop.
While the connection is live,
devices attached to the host afterwards appear too,
and detached devices disappear.


Verify the connection
---------------------

.. @artefact workshop connections

List the connections to confirm the plug is wired to the slot.
The :option:`!--all` flag includes disconnected plugs:

.. code-block:: console

   $ workshop connections --all

     INTERFACE      PLUG                        SLOT                      NOTES
     custom-device  dev/input-sdk:input-device  dev/system:custom-device  manual


The :samp:`manual` note marks a connection made by hand
rather than one established automatically at launch.


Access the devices
------------------

.. @artefact workshop shell

Open a shell in the workshop
and list the devices from the connected subsystem:

.. code-block:: console

   $ workshop shell dev
   workshop@dev:~$ ls /dev/input/

     event0  event1  mice


The host's :samp:`input` devices are now reachable inside the workshop.

To revoke access, disconnect the plug:

.. @artefact workshop disconnect

.. code-block:: console

   $ workshop disconnect dev/input-sdk:input-device


See also
--------

Explanation:

- :ref:`exp_custom_device_interface`
- :ref:`exp_plugs_slots`


How-to guides:

- :ref:`how_declare_plugs_slots`
- :ref:`how_use_multiple_workshops`


Reference:

- :ref:`ref_workshop_connect`
- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_disconnect`
- :ref:`ref_workshop_shell`
