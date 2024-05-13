.. _exp_interface_connections:

Interface connections
=====================

For a workshop to be operational,
the plugs defined by the SDKs listed in the workshop definition
connect to respective :ref:`interface slots <exp_interfaces>`
at some point.

Such *connections* are established uniformly via a special SDK
that is quietly present in any workshop
but not immediately exposed to its users.


Basics
------

Interface connections provide a communication and resource sharing mechanism.
It is integral to workshop confinement,
ensuring that each workshop operates within its own isolated environment
while still allowing controlled interactions with system resources.

Here's how it works on the outside:

- The :command:`workshop connect` command establishes a connection
  between a workshop and a system interface,
  allowing the workshop to access system resources securely.

- Conversely, the :command:`workshop disconnect` command
  terminates existing connections between a workshop and a system interface,
  revoking access to system resources granted by the connection.

- Finally, the :command:`workshop connections` command
  lists any existing connections and their states,
  providing an overview of how workshops are communicating with the system.

Some plugs can be auto-connected to their slots at launch or refresh.
This behaviour varies by interface,
but the overall goal is to provide a reasonably seamless, logical experience.
For example, content interface plugs are auto-connected,
whereas an SSH interface plug requires manual connection.


Agent SDK
---------

Every workshop contains an *agent SDK*
that exposes system resources through interface slots.
Essentially, it is a special SDK type,
which is not available in the SDK Store but is auto-added to each workshop.
It is installed first at :command:`workshop launch`
and removed last at :command:`workshop remove`,
ensuring internal consistency.

The goal of the agent SDK isn't to offer hooks or provide additional content;
it exists solely to expose system resources to other SDKs in a uniform fashion.
As such, it cannot be removed by the user
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
