.. _exp_sdk:

SDKs
====

SDKs are essential workshop components
that install the required system and language packages,
configure the workshop for their operation
and maintain their own state
throughout the lifetime of the workshop.
An *SDK* is designed by a publisher
and made available via the SDK Store.
A single workshop can include multiple SDKs from different publishers.
SDKs are distributed through channels similar to
`snap channels <https://snapcraft.io/docs/channels>`_.


.. _exp_sdk_state:

SDK state
---------

An SDK can store any data specific to it,
such as a model training configuration,
within the workshop.
To enable this,
the SDK publisher implements save and restore :ref:`hooks <exp_sdk_hooks>`
that |project_markup| runs at the appropriate moments
to consistently handle such data, collectively known as *SDK state*.

For example, before changes are applied to the workshop
during :command:`workshop refresh`,
the states of the SDKs are saved
by invoking their :samp:`save-state` hooks.
On success,
they are restored using the :samp:`restore-state` hooks.


.. _exp_sdk_definition:

SDK definition
--------------

An SDK is defined by the SDK publisher;
the definition may look like this:

.. code-block:: yaml
   :caption: sdkcraft.yaml

   name: go
   title: Go SDK
   base: ubuntu@20.04
   summary: The Go programming language
   description: |
     Go is an open source programming language that enables the production
     of simple, efficient and reliable software at scale.

   plugs:
     mod-cache:
       interface: content
       target: /home/workshop/go/pkg/mod


.. _exp_sdk_hooks:

SDK hooks
---------

SDK publishers can define optional *hooks*
that control and extend the workshop's internal behaviour
to make any framework wrapped as an SDK
compatible with |project_markup|'s logic;
in particular, the hooks manage the SDK state
and report its health.

Each hook is a shell script with domain-aware actions
that |project_markup| runs in the workshop
at a particular life cycle stage
to ensure that the SDK stays functional.
Specific examples include :samp:`setup-base`,
:samp:`save-state` and :samp:`restore-state`.

You may see individual hooks mentioned in the output of
:command:`workshop changes` and :command:`workshop tasks`;
understanding the events that trigger them can help you with troubleshooting.


.. _exp_interfaces:

Interfaces
----------

To make SDKs customisable and extensible,
|project_markup| implements a counterpart to
:program:`snapd`'s
`interface manager <https://snapcraft.io/docs/interface-management>`__,
which controls whether an SDK can use resources beyond its confines.
You can think of specific interfaces as resource *types*:
file system, hardware, computing and so on.
The interfaces are referenced by the SDKs,
so the user doesn't have direct control over them in the workshop definition.

Currently, |project_markup| supports the following interfaces:

- :ref:`content interface <exp_content_interface>` (auto-connected)
- :ref:`GPU interface <exp_gpu_interface>` (auto-connected)
- :ref:`SSH interface <exp_ssh_interface>` (manually connected)


.. _exp_plugs_slots:

Plugs and slots
~~~~~~~~~~~~~~~

To provide access to these resource types,
|project_markup| exposes *interface slots*.
For example, a :ref:`content interface <exp_content_interface>` slot
creates an internal host directory to be mounted inside the workshop;
think of the slot as the provider of the resource.

Further, individual SDKs define *plugs*
to connect to a slot of a certain interface type.
In our :ref:`definition example <exp_sdk_definition>`,
it's the *content interface* mentioned above.

You can think of the plug as the recipient of the resources exposed by the slot;
note that a slot can handle connections with multiple plugs.

This mechanism comes into play when you
:command:`workshop launch` or :command:`workshop start` the workshop;
the plugs defined by its SDKs are automatically connected to the slots,
provided that the definition has all |project_markup| needs to make a match.


.. _exp_interfaces_validation:

Validation
~~~~~~~~~~

To ensure plugs can be installed and connected,
|project_markup| uses a set of rules,
with each interface having its own.
For example, the content interface plug can be installed and auto-connected
based on its rules alone.
However, other interfaces may have different rules,
such as allowing installation but not auto-connection for :samp:`ssh-agent`.

Finally, once all the checks are done,
the SDKs are ready to use the external resources.


.. _exp_interfaces_cli_operations:

Related CLI operations
~~~~~~~~~~~~~~~~~~~~~~

A number of basic workshop operations
affect plugs and slots in different ways.

When you :command:`workshop launch` a workshop,
an auto-connect task handles each interface plug,
finding a candidate slot,
verifying the plug's eligibility for the slot based on their declarations
and connecting the two.

On :command:`workshop refresh`,
existing connections are preserved in the refreshed workshop
if their plugs were connected before the operation.
A newer version of an SDK may drop a plug that was previously connected;
such connections are removed,
but the host-based content remains.

On :command:`workshop remove`,
both the interface connections and the default host directories
(if any have been created, for example, to accommodate content interface slots)
are removed.

.. note::

   We remove content stored in our default locations
   because it's not a good idea to keep user data forever.
   Thus, at least some commands will delete this data
   to prevent it from piling up in hidden places
   where it's unlikely to be used again.


Also, you can manually enable or disable connections
with :command:`workshop connect` and :command:`workshop disconnect`,
whereas :command:`workshop connections` can list all connections
that have been established by any |project_markup| projects.


See also
--------

Explanation:

- :ref:`exp_projects`
- :ref:`exp_workshop`


Reference:

Reference:

- :ref:`ref_workshop_connect`
- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_changes`
- :ref:`ref_workshop_disconnect`
- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_start`
- :ref:`ref_workshop_tasks`
- :ref:`ref_sdk_hooks`
