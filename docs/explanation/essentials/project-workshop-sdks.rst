Project, workshop, SDKs
=======================

Projects, workshops, workshop definitions and SDKs
are the key building blocks of |project|.


Introduction
------------

To start using |project|,
it is important to understand how these concepts fit together.

You can view a *project* as your working directory,
doing all the things you would usually do there:
create and populate repositories, write and build code, run models, and so on.
However, the difference starts with the software dependencies
you would earlier install as system-wide packages, container images,
or in myriad other ways.
Instead, they are wrapped and published as |project|-ready, isolated *SDKs*
which you list while defining a *workshop*.

A single workshop always points to a project,
and a project may have multiple workshops referencing it,
with each workshop containing a number of SDKs.

What do you get out of this multitude?

First, |project| is transactional in nature;
you won't have to trace residual files and libraries all across your system
after you uninstall a package that turned out too unstable to your taste.
Even if an SDK dumps something unexpected onto the disk,
it's contained within the workshop.
|project| aims to encapsulate each part of functionality you may need,
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
create a :ref:`workshop definition <exp_workshop_def>` in it
and run :ref:`ref_workshop_launch`.
Launching a workshop from a project
establishes the relationship between these two,
which is required to actually start a workshop.

When the workshop is then started with :ref:`ref_workshop_start`,
the project is mounted to it as :file:`/project/`,
and the :ref:`ref_workshop_stop` command unmounts it.

.. note::

   There are more :ref:`workshop <ref_workshop_cli>` commands;
   some have a :option:`!--project` option
   that accepts a pathname to use as the project directory.

External changes to the project are tracked by the |project| daemon.
Thus, if the project moved or copied,
all workshops referencing it are updated
so you can continue working uninterrupted.

If the project is deleted by external means,
workshops still referencing it
enter the *Error* state;
the only applicable command will be :ref:`ref_workshop_remove`.


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

An SDK has a state that persists SDK-specific data,
such as a model training configuration.
|project| saves the state before applying any changes to the SDK,
which usually arrive with a :ref:`ref_workshop_refresh` operation.
After a successful change, the state is restored.
The specific save and restore actions
are implemented by the publisher.
