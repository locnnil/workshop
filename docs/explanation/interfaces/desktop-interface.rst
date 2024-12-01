.. _exp_desktop_interface:

Desktop interface

The desktop interface
provides access to the host system's Wayland socket
from inside the workshop, allowing it to securely
execute Wayland GUI applications.

By using the interface,
the SDK publisher allows the workshop to access the host's Wayland socket,
which can be useful for various SDK-specific tasks such as
building graphical applications or using editors without remote support.

The interface isn't connected automatically at launch and refresh
for security reasons.
The :command:`workshop connect` and :command:`workshop disconnect` commands
can be invoked manually after the workshop has started:

.. code-block:: console

   $ workshop connect ws/desktop-sdk:desktop
   $ workshop disconnect ws/desktop-sdk:desktop


Establishing a connection means
a proxy Unix domain socket has been created
and the following environment variables have been set:

- :envvar:`WAYLAND_DISPLAY`
  Identifies the name of the Wayland socket
- :envvar:`XDG_SESSION_TYPE`
  Specifies the current display server type
- :envvar:`QT_QPA_PLATFORM`
  Specifies the Qt platform plugin to be used for Qt-based applications
- :envvar:`ELECTRON_OZONE_PLATFORM_HINT`
  Specifies the preferred platform for electron applications

To check if the interface is connected:

.. code-block:: console

   $ workshop connections --all

     Interface  Plug                   Slot       Notes
     ...
     desktop    ws/desktop-sdk:desktop :desktop   manual

This means the host's Wayland socket is available inside the workshop

.. code-block:: console

   $ workshop shell ws
   workshop@ws-8584e571$ ls $XDG_RUNTIME_DIR | grep wayland

     wayland-1


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
- :ref:`ref_workshop_shell`
