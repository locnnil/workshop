Interfaces
==========

These articles explain concepts
that are important for understanding |project_markup|'s interface mechanics.

.. toctree::
   :glob:
   :maxdepth: 1

   *


.. _exp_interface_connections:

Summary
-------

For a workshop to be operational,
the plugs defined by the SDKs listed in the workshop definition
must at some point *connect* to the appropriate interface slots.

Such connections are uniformly established via the
:ref:`agent SDK <exp_agent_sdk>`
that is quietly present in every workshop,
but not immediately visible to its users.

Interface connections are a mechanism for communication and resource sharing.
It is an integral part of workshop confinement,
ensuring that each workshop operates in its own isolated environment,
while still allowing controlled interactions with system resources.

Here's how it works from the outside:

- The :command:`workshop connect` command establishes a connection
  between a workshop and a system interface,
  allowing the workshop to securely access system resources.

- Conversely, the :command:`workshop disconnect` command
  terminates existing connections between a workshop and a system interface,
  revoking the access to system resources granted by the connection.

- Finally, the :command:`workshop connections` command
  lists all existing connections and their states,
  providing an overview of how workshops are communicating with the system.

Some plugs can be auto-connected to their slots at launch or refresh.
This behaviour varies by interface,
but the overall aim is to provide a reasonably seamless, logical experience.
For example, content interface plugs are auto-connected,
whereas an SSH interface plug requires manual connection.


.. _exp_agent_sdk:

Agent SDK
~~~~~~~~~

Every workshop contains an *agent SDK*
that exposes system resources through interface slots.
It's essentially a special SDK type,
which is not available from the SDK Store but is auto-added to each workshop.
It's installed first at :command:`workshop launch`
and removed last at :command:`workshop remove`,
ensuring internal consistency.

The purpose of the agent SDK isn't to add hooks or additional content;
it's only there to expose system resources to other SDKs in a consistent way.
As such, it can't be removed by the user
and isn't listed in the :command:`workshop info` output.


See also
--------

Explanation:

- :ref:`exp_sdk_definition`
- :ref:`exp_plugs_slots`
- :ref:`exp_workshop_def`


Reference:

- :ref:`ref_workshop_connect`
- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_disconnect`
- :ref:`ref_workshop_info`
- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_remove`
