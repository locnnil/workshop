.. _exp_workshop_concepts:

.. meta::
   :description: Overview of workshop-related concepts, explaining the role of
                 workshops as containers defined by project directories and
                 hosted by LXD for consistent development environments.

Workshop concepts
=================

.. @artefact project
.. @artefact workshop (container)
.. @artefact workshop definition

A *workshop*
(lowercase; not to be confused with |ws_markup| itself)
is a container that enables consistent environment builds.
A workshop is defined by a single YAML file
that acts as the blueprint for |ws_markup| to implement at launch time.
It describes how individual components fit together
to create a cohesive development environment.
A *project* is the working directory where workshop definitions are placed.
When you start a workshop, the project directory is mounted inside it,
so storing repositories, code, or data such as models in the project directory
enables you to use them inside the workshop.

Currently, these containers are hosted by `LXD`_,
but it's not recommended to rely on this implementation detail.


.. _exp_workshop_status:

Workshop status
---------------

.. @artefact workshop status

A workshop's lifecycle can see it switch between several statuses:

.. list-table::
   :header-rows: 1

   * - State
     - Description

   * - *Off*
     - Just defined, not operational;
       the workshop container does not exist yet.

   * - *Ready*
     - Operational;
       the workshop container is running and ready for use.

   * - *Stopped*
     - Operational;
       the workshop container is stopped and can be restarted.

   * - *Pending*
     - Not operational;
       the workshop container is running
       but is being updated and is not ready for use.

   * - *Waiting*
     - Operational;
       the workshop container is running and available for command execution,
       typically for debugging a launch or refresh error;
       the current :ref:`change <exp_changes_tasks>` is in progress.

   * - *Error*
     - Not operational;
       the workshop is in a non-functional state due to an error.


Status diagrams in the `See also`_ section below
provide more details of valid transitions.


.. _exp_workshop_definition:

Workshop definition
-------------------

.. @artefact workshop base image
.. @artefact workshop definition

This is a YAML file
that lists the base image of the workshop
and the specific components installed on top of it.
The definition acts as a single source of truth about the workshop.
It usually takes a few tries
to produce a definition that works for your project,
so you can edit and update the file iteratively.

A simple definition might look like this:

.. code-block:: yaml
   :caption: workshop.yaml

   name: dev
   base: ubuntu@22.04
   sdks:
     - name: go
       channel: 22.04/stable


.. @artefact SDK
.. @artefact interface

It specifies a *base* and an *SDK*.
A more complete definition would usually list several SDKs
that use different :ref:`interfaces <exp_interface_concepts>`,
software packages, and :ref:`hooks <exp_sdk_hooks>`.


.. _exp_base:

Base image
----------

The base specifies the underlying operating system image,
such as a particular Ubuntu LTS release.
This is the first layer of the workshop,
upon which all other components are applied.

For details on how the images are handled behind the scenes,
see :ref:`exp_arch_images`.


.. _exp_workshop_definition_sdks:

SDKs
----

The :samp:`sdks` section brings in the features and tools,
layering them on top of the base image.
Each SDK listed here is a bundle of code, data, and configurations,
pre-packaged with |sdk_markup| to be used with |ws_markup|;
see :ref:`exp_sdk_concepts` for details.

This layering is not just conceptual;
at launch time,
|ws_markup| uses ZFS snapshots to separate the SDKs:

#. The :samp:`base` OS is installed.

#. The :samp:`system` SDK is installed,
   and its :samp:`setup-base` hook is run.

#. A ZFS snapshot is taken,
   and cloned to create a new ZFS file system.

#. For each subsequent SDK
   in the order of their appearance on the :samp:`sdks` list,
   its :samp:`setup-base` hook is run
   and another snapshot is taken and cloned.


This will create a chain of snapshots,
where each one represents a cumulative layer of the workshop.
Snapshots makes operations like refreshing or reverting a workshop very fast,
as |ws_markup| can simply restore a previous snapshot
instead of rebuilding the environment from scratch.
No snapshots are created for other hook types,
such as :samp:`setup-project` or :samp:`save-state`.

In order to restore an old snapshot,
newer snapshots must be destroyed first.
If refreshing fails,
the workshop reverts to its previous state.
The cloned file systems are used to restore the deleted snapshots.

For details on how |ws_markup| leverages ZFS,
see :ref:`exp_arch_zfs_storage`.


.. _exp_workshop_definition_connections:

Plugs, slots, connections
-------------------------

.. @artefact interface plug
.. @artefact interface slot
.. @artefact interface connection

Once all the SDKs are installed,
they often need to communicate with each other or with the host system.
This is handled by establishing interface connections
between plugs (service consumers) and slots (service providers);
see :ref:`exp_interface_concepts` for details.

These plugs and slots can be defined in two ways:

- By the SDK itself:
  An SDK can define its own plugs and slots in its :file:`sdk.yaml` file.
  These are the standard capabilities the SDK offers.

- Grafted by the workshop:
  A workshop definition can add plugs or slots to an SDK it references.
  This is done within an SDK's entry in the :file:`workshop.yaml` file.
  Grafting allows for customizing or extending an SDK's capabilities locally,
  without modifying the original SDK;
  this reduces the need to run manual commands after starting the workshop.


The :samp:`connections` section of the definition can explicitly link
any plugs and slots available within the workshop,
on top of what the :ref:`auto-connection mechanism <exp_interface_connections>`
in |ws_markup| provides:
eventually, all interface connections are
:ref:`resolved, validated, and established <exp_interfaces_validation>`
in a single task *after* all the SDK layers have been created,
because all components must be in place before the wiring can be done.

This example adds a slot, a plug, and two connections to its SDKs:

.. code-block:: yaml
   :caption: .workshop/dev.yaml
   :emphasize-lines: 6-9, 12-14

   base: ubuntu@22.04
   name: dev
   sdks:
     - name: tensorflow
       channel: 22.04/stable
       plugs:
         cuda:
           interface: mount
           workshop-target: /usr/local/cuda/lib64
     - name: imagenet
       channel: 22.04/stable
       slots:
         images:
           interface: mount
           workshop-source: $SDK/images
     - name: cuda
       channel: 22.04/stable
   connections:
     - plug: tensorflow:cuda
       slot: cuda:libs


This extends the :samp:`tensorflow` SDK
with a standard path for CUDA runtime libraries.
In :samp:`connections`,
we explicitly connect the :samp:`cuda` plug,
newly defined under the :samp:`tensorflow` SDK,
to the :samp:`libs` slot from the :samp:`cuda` SDK.
Thus, upon workshop creation,
the plug will be connected
not to a default system SDK location on the host
(for example, :file:`.../<ID>/<WORKSHOP>/...`),
but to a library path *inside* the workshop,
which is set by :samp:`workshop-target`.

Mind that the connection established in this way
is no different from those created via the command line.


.. _exp_workshop_definition_actions:

Actions
-------

.. @artefact workshop actions

Another optional part of a workshop definition is the :samp:`actions` section;
it contains named shell scripts to be copied and executed inside the workshop.
This section provides a degree of convenience,
allowing the users to define simple aliases
for longer or more complex shell commands
that they expect to run frequently inside the workshop,
right in the definition file.

Actions are not part of the layered snapshot system at all.
They stay in the definition,
and are parsed by the :ref:`daemon <exp_arch_daemon>`
every time the :command:`workshop run` command is executed.
This means the users can add or modify actions and use them immediately,
without needing to refresh or restart the workshop.

The following example adds four actions,
:samp:`lint`, :samp:`shellcheck`, :samp:`unit`, and :samp:`cover`,
intended as utility helpers for a development environment:

.. code-block:: yaml
   :caption: .workshop/dev.yaml
   :emphasize-lines: 6-15

   name: dev
   base: ubuntu@24.04
   sdks:
     - name: go
       channel: 22.04/stable
   actions:
     lint: |
       golangci-lint run  --out-format=colored-line-number -c .golangci.yaml
     shellcheck: |
       git ls-files | file --mime-type -Nnf- | grep shellscript | cut -f1 -d: | xargs shellcheck
     unit: |
       go test ./...
     cover: |
       go test ./... -coverprofile=coverage.out
       go tool cover -html=coverage.out


To run these actions, you use the :command:`workshop run` command:

.. code-block:: console

   $ workshop run lint


When you thus invoke an action, it's injected into the workshop
and executed there in a fashion similar to :command:`workshop exec`.
Even if you update the :samp:`actions` section in the definition,
there's no need to refresh the workshop to use the updated action;
it's available immediately.

For a quick reference of the actions in your workshop,
run :command:`workshop actions`:

.. code-block:: console

   $ workshop actions


This mechanism avoids the need to maintain helper scripts manually,
ensuring instead that they are stored with the rest of the workshop's metadata.


Origins and locations
---------------------

.. @artefact system SDK
.. @artefact sketch SDK
.. @artefact in-project SDK

Workshop components, including the many SDK types,
originate from different sources
and end up in multiple locations.
The workshop definition file acts as a blueprint
that brings these distributed components together:

.. list-table::
    :header-rows: 1
    :widths: 10 25 30 35

    * - Component
      - Origin
      - Storage location
      - Description

    * - :ref:`Workshop definition <exp_workshop_definition>`
      - Created manually in YAML by |ws_markup| users
      - Project directory on the host
      - Defines the workshop environment
        and how it should be built and run.

    * - :ref:`System SDK <exp_system_sdk>`
      - Built into |ws_markup|
      - Automatically exposed in the workshop at launch
      - Provides host system integration capabilities
        (mounts, camera, GPU, networking, and so on).

    * - :ref:`Regular SDKs <exp_sdk_concepts>`
      - Distributed via the SDK Store
        with :samp:`channel` versioning
      - Downloaded, cached on the host,
        and installed in the workshop at launch
      - These SDKs are the most common variation,
        providing tools and libraries from external publishers.

    * - :ref:`In-project SDKs <exp_in_project_sdk>`
      - Created manually or ejected with :command:`workshop sketch-sdk --eject`
      - Defined in the project directory on the host;
        installed in the workshop at launch
      - Custom SDKs, specific to the workshop;
        these are defined within the project directory
        and can be identified by the :samp:`project-` prefix in their names
        in the workshop definition.

    * - :ref:`Sketch SDK <exp_sketch_sdk>`
      - Generated with :command:`workshop sketch-sdk`
      - Defined under :file:`$XDG_DATA_HOME/workshop/`;
        installed in the workshop at refresh
      - Encapsulates local, transient logic
        in an SDK that can be quickly iterated upon
        and later ejected as an in-project SDK.

    * - :ref:`Actions <exp_workshop_definition_actions>`
      - Defined by |ws_markup| users
      - Listed directly in the workshop definition
      - Utility scripts, specific to the workshop;
        these are injected into the workshop at run time.


See also
--------

Explanation:

- :ref:`exp_interfaces`
- :ref:`exp_projects`
- :ref:`exp_sdks`


How-to guides:

- :ref:`how_use_workshops`


Reference:

- :ref:`ref_workshop__cli`
- :ref:`ref_workshop_actions`
- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_definition`
- :ref:`ref_workshop_run`
- :ref:`ref_workshop_status`
