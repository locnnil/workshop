
.. _exp_arch_runtime_behavior:

.. meta::
   :description: Explanation article on the runtime behavior of Workshop,
                 detailing the workshop launch process, data architecture,
                 ZFS storage, and state database management.

Runtime behavior
================

This section demonstrates how the system components of |ws_markup|
work together to transform a workshop definition into a running container.
It focuses on the dynamic processes and workflows
that occur during workshop operations.

.. note::

   For a general description of these components,
   see :ref:`exp_arch_system_components`.


.. _exp_arch_workshop_launch:

Workshop launch process
-----------------------

The workshop launch process coordinates the 
:ref:`workshopd daemon <exp_arch_daemon>`,
:ref:`LXD backend <exp_arch_lxd_backend>`,
:ref:`state management <exp_arch_state_database>`,
:ref:`interface policy <exp_arch_interface_system>`,
and :ref:`ZFS storage <exp_arch_zfs_storage>` subsystems,
all detailed in the previous section.

The launch sequence begins with workshop YAML validation and normalization.
The LXD container is then created from a base image.
The :ref:`system SDK <exp_system_sdk>` is installed first to provide
the core integration layer with host system resources.

Regular SDKs are installed sequentially.
SDK-specific :ref:`setup hooks <exp_sdk_hooks>` are executed during this phase.
Workshop creates a lightweight ZFS clone after each SDK installation to enable efficient updates and rollbacks.
Interface validation performs compatibility checking and policy enforcement.
Connection establishment handles device attachment and resource binding.
The container is then started and health checks are performed to complete the launch.

- Launch operations create new workshops from scratch
  by creating new LXD containers and projects as needed,
  downloading and caching base images as required,
  installing all SDKs with complete hook execution,
  and establishing all interface connections.

- Refresh operations update existing workshops
  by preserving LXD container identity and networking,
  restoring to base snapshot before applying changes,
  skipping unchanged SDKs using snapshot comparison,
  running save-state and restore-state hooks for SDK data persistence,
  and updating only modified interface connections.

Workshop status transitions follow a predictable lifecycle:

- *Off* → *Pending*:
  Workshop creation is initiated by user request

- *Pending* → *Waiting*:
  Launch encounters errors requiring manual intervention

- *Pending* → *Stopped*:
  Launch succeeds but the container remains stopped

- *Stopped* → *Ready*:
  Container starts successfully and becomes available for use


These changes are tracked in the state management system
and can be monitored through the API.

.. note::

   For a complete reference guide on status transitions,
   see :ref:`ref_workshop_status`.


.. _exp_arch_container_layout:

Container layout
----------------

Container runtime setup builds on the 
:ref:`LXD backend <exp_arch_lxd_backend>` foundation
with runtime-specific configuration applied during workshop launch.

Key directories inside the container include:

- The root file system as a ZFS dataset.

- The :file:`/project/` mount
  to provide transparent access to project files on the host.

- The :file:`/var/lib/workshop/` directory
  where workshop state volume and SDK volumes are mounted.


The :ref:`interface system <exp_arch_interface_system>` provisions
different resources within containers:

- Mount interfaces appear as regular directories.
- Proxy devices handle port forwarding and services such as the SSH agent.
- GPU, audio, and camera devices are passed through to the workshop
  with proper permissions and access controls.


Diagrams
--------

Workshop launch flow:

.. mermaid::
  :alt: Sequence diagram showing the workshop launch flow from CLI command
      through daemon processing, LXD container creation, SDK installation,
      and interface connection establishment.
  :align: center
  :zoom:

  sequenceDiagram
    participant CLI
    participant API as workshopd API
    participant Daemon
    participant TaskRunner as Task runner
    participant LXD as LXD backend
    participant ZFS as ZFS storage
    participant Interfaces as Interface system

    CLI->>API: POST /v1/projects/{id}/workshops (workshop.yaml)
    API->>Daemon: Create launch change
    Daemon->>TaskRunner: Queue launch tasks

    TaskRunner->>LXD: Create workshop container
    LXD->>ZFS: Create root filesystem
    ZFS-->>LXD: Filesystem ready
    LXD-->>TaskRunner: Container created

    TaskRunner->>LXD: Install system SDK
    LXD->>ZFS: Take base snapshot
    ZFS-->>LXD: Snapshot created
    LXD-->>TaskRunner: System SDK installed

    loop For each SDK in definition
      TaskRunner->>LXD: Install SDK
      LXD->>LXD: Run setup-base hook
      LXD->>ZFS: Take SDK snapshot
      ZFS-->>LXD: Snapshot created
      LXD-->>TaskRunner: SDK installed
    end

    TaskRunner->>Interfaces: Validate connections
    Interfaces->>Interfaces: Check plug/slot compatibility
    Interfaces-->>TaskRunner: Validation complete

    TaskRunner->>LXD: Establish interface connections
    LXD->>LXD: Configure container devices
    LXD-->>TaskRunner: Connections established

    TaskRunner->>LXD: Start workshop container
    LXD-->>TaskRunner: Container started

    TaskRunner-->>Daemon: Launch complete
    Daemon-->>API: Change finished
    API-->>CLI: HTTP 200 (workshop ready)
