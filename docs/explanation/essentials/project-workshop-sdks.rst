Project, workshop, SDKs
=======================

Projects, workshops and their SDKs
are the key building blocks of |project_markup|.


Introduction
------------

To start using |project_markup|,
it is important to understand how these concepts fit together.

You can view a *project* as your working directory,
doing all the things you would usually do there:
create and populate repositories, write and build code, run models, and so on.
However, the difference starts with the software dependencies
you would earlier install as system-wide packages, container images,
or in myriad other ways.
Instead, they are wrapped and published as |project_markup|-ready, isolated *SDKs*
which you list while defining a *workshop*.

A single workshop always points to a project,
and a project may have multiple workshops referencing it,
with each workshop containing a number of SDKs.

What do you get out of this multitude?

First, |project_markup| is transactional in nature;
you won't have to trace residual files and libraries all across your system
after you uninstall a package that turned out too unstable to your taste.
Even if an SDK dumps something unexpected onto the disk,
it's contained within the workshop.
|project_markup| aims to encapsulate each part of functionality you may need,
keeping things clean and tidy.

Next, it's portable;
imagine sending a *compact* tarball of your project to a coworker
who then recreates it with exactly the same dependency versions that you used.
What's better, this is achieved without manually customised,
high-maintenance image definitions or configurations;
all the work of keeping the SDKs in your workshop operational
is done by the people who are actually responsible for it
(namely, the publishers).


.. _exp_project:

Project
-------

Technically, a project is a directory
that contains one or many workshop definitions.

To initialise a directory as a project,
create a
:ref:`workshop definition <exp_workshop_def>`
in it
and run :command:`workshop launch`.
Launching a workshop from a project
establishes the relationship between these two,
which is required to actually start a workshop.

When the workshop is then started with :command:`workshop start`,
the project is mounted to it as :file:`/project/`,
and the :command:`workshop stop` command unmounts it.

.. note::

   There are more workshop CLI commands;
   some have a :option:`!--project` option
   that accepts a pathname to use as the project directory.

External changes to the project are tracked by the |project_markup| daemon.
Thus, if the project moved or copied,
all workshops referencing it are updated
so you can continue working uninterrupted.

If the project is deleted by external means,
workshops still referencing it
enter the *Error* state;
the only applicable command will be :command:`workshop remove`.


.. _exp_workshop:

Workshop
--------

A *workshop* (lowercase) is a container that is described in a definition file.
Currently, these containers are hosted by
`LXD <https://documentation.ubuntu.com/lxd/en/latest/>`__,
but relying on this implementation detail isn't recommended.


.. _exp_workshop_def:

Workshop definition
~~~~~~~~~~~~~~~~~~~

This is a file named :file:`.workshop.<NAME>.yaml`
that lists the base image of the workshop
and the specific components installed on top of it.
The definition serves as a single source of truth about the workshop.
It usually takes a few tries
to arrive at a configuration that suits your project,
so you can edit and update the workshop definition iteratively.

A simple definition may look like this:

.. code-block:: yaml
   :caption: .workshop.golang.yaml

   name: golang
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/stable

This specifies a *base* and an *SDK*.
A more complete definition would usually list
multiple SDKs, interfaces, packages and life cycle hooks.


.. _exp_workshop_base:

Base image
~~~~~~~~~~

The base is a supported OS image
that is used as the foundation of the workshop.
Currently, it can be either ``ubuntu@20.04`` or ``ubuntu@22.04``.


.. _exp_sdk:

SDKs
----

SDKs are essential workshop components
that install the required system and language packages,
configure the workshop for their operation
and maintain their own state
throughout the lifetime of the workshop.
An *SDK* is designed by a publisher
and made available via the SDK Store.
A single workshop can include multiple SDKs from different publishers.
SDKs are distributed via channels similar to
`snap channels <https://snapcraft.io/docs/channels>`_.


.. _exp_sdk_state:

SDK state
~~~~~~~~~

An SDK may store any data specific to it,
such as a model training configuration,
within the workshop.
The publisher of the SDK implements save and restore actions
to let |project_markup| handle such data consistently as the *SDK state*.

Before applying any changes to the SDK,
usually during a :command:`workshop refresh`,
|project_markup| saves the workshop's SDK states
by invoking their save actions.
After a successful change,
the states are respectively restored.


.. _exp_sdk_def:

SDK definition
~~~~~~~~~~~~~~

An SDK is defined in a file named :file:`sdk.yaml` that may look like this:

.. code-block:: yaml
   :caption: sdk.yaml

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


Interfaces
~~~~~~~~~~

To make SDKs customisable and extensible,
|project_markup| implements a counterpart to
:program:`snapd`'s
`interface manager <https://snapcraft.io/docs/interface-management>`__,
controlling whether an individual SDK can use resources beyond its confinement.
You can think of specific interfaces as resource *types*:
file system, hardware, computational and so on.


.. _exp_interfaces_plugs_slots:

Plugs and slots
^^^^^^^^^^^^^^^

In order to provide access to these resource types,
|project_markup| exposes so-called *interface slots*.
For instance, a :ref:`content interface slot <exp_content_interface>`
creates a designated host directory to be mounted inside the workshop;
think of the slot as the provider of the resource.

On top of that, individual SDKs define *plugs*
to connect to a slot that belongs to a certain interface.
In our :ref:`previous example <exp_sdk_def>`,
it's the aforementioned *content interface*.

You can think of the plug as the recipient of the resources exposed by the slot;
note that a slot can handle connections with multiple plugs.

Eventually, this mechanism starts whirring when the workshop itself is started;
the plugs defined by its SDKs are automatically connected to the slots,
provided the definition contains everything |project_markup| needs to make a match.


.. _exp_interfaces_validation:

Validation and policies
^^^^^^^^^^^^^^^^^^^^^^^

Now, to make sure plugs can be installed and auto-connected,
|project_markup| uses a set of rules called policies,
with each interface having its own.
For example, the content interface plug can be installed and auto-connected
based on its policy alone.
However, other interfaces may have different rules,
such as enabling installation but not auto-connection for :samp:`ssh-agent`.

Finally, when all checks are done,
the SDKs are able to use the external resources.


.. _exp_interfaces_cli_operations:

Related CLI operations
^^^^^^^^^^^^^^^^^^^^^^

A number of basic workshop operations
affect plugs and slots in different ways.

When you :command:`launch` a workshop,
an auto-connect task handles the content interface plug,
finding a candidate slot,
verifying the plug's eligibility for the slot based on their declarations
and connecting the two.

Upon :command:`refresh`,
existing connections are preserved in the refreshed workshop
if their plugs were connected before the operation.
A newer version of an SDK may drop a plug that was previously connected;
such connections are removed,
but the content remains.

On :command:`remove`,
both the interface connections and the host directories
(if any were created, for example, to accommodate content slots)
are removed.

.. note::

   We remove the content from the default locations
   because it's not a good idea to keep user data forever.
   Thus, at least some workshop operations will delete this data
   to prevent it from piling up in hidden locations,
   where it's unlikely to be used again.


See also
--------

Reference:

- :ref:`ref_workshop_cli`
- :ref:`workshop launch (command) <ref_workshop_launch>`
- :ref:`workshop refresh (command) <ref_workshop_refresh>`
- :ref:`workshop remove (command) <ref_workshop_remove>`
- :ref:`workshop start (command) <ref_workshop_start>`
- :ref:`workshop stop (command) <ref_workshop_stop>`