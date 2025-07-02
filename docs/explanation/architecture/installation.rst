.. _exp_arch_install:

Initial installation
====================

.. @artefact installation

|ws_markup| is designed to run on Linux systems.
It is primarily distributed as a `snap <https://snapcraft.io/>`_
and relies on LXD as its containerization backend
and ZFS for efficient storage management.


Main components
---------------

.. @artefact workshop (CLI)
.. @artefact workshopd
.. @artefact workshopctl

|ws_markup| installation on your host system includes three primary components:

- The :program:`workshop` CLI serves as the main user interface,
  providing a thin client that translates user commands
  and communicates with :program:`workshopd` through a Unix domain socket.

- The :program:`workshopd` daemon runs as a background process
  with elevated privileges,
  managing the complete workshop life cycle
  including container creation,
  SDK installation,
  interface coordination
  and state persistence through a REST API.

- The :program:`workshopctl` tool runs inside a workshop
  and has access to a very limited subset of :program:`workshopd`'s REST API
  for service-related and reporting tasks.


The communication architecture between these components uses a layered approach
where the CLI communicates with :program:`workshopd` via Unix domain sockets,
while :program:`workshopd` interfaces with LXD through the latter's native API.


CLI
~~~

.. @artefact workshop (CLI)

The :program:`workshop` CLI provides a comprehensive command-line interface
for managing workshops and interacting with :program:`workshopd`.
It is organized into logical command groups
for different aspects of |ws_markup|'s operations:

- Create, update, and delete operations
- Start and stop control
- Exploration and troubleshooting
- Interface connection management
- Workshop utilization
- SDK sketching
- Miscellaneous utilities


For all operations, the CLI communicates with :program:`workshopd` through a REST API
exposed over Unix domain sockets,
using a client library that handles connection management,
error handling, and request-response serialization.
The client supports both synchronous and asynchronous operations,
with proper error reporting and progress tracking.


Daemon
~~~~~~

.. @artefact workshopd

The :program:`workshopd` is the core component of |ws_markup|
responsible for managing the complete workshop life cycle. It uses LXD as its container backend.

The :program:`workshopd` daemon is a :program:`systemd` service
responsible for the complete workshop life cycle.
At its core, it relies on the proven state package from :program:`snapd`
to ensure that any changes to a workshop are handled in a transactional manner:
safely, consistently, and reversibly.

When integrated with :program:`systemd`,
:program:`workshopd` implements the watchdog notification mechanism for health monitoring and supports socket activation to reduce memory footprint.


REST API
^^^^^^^^

.. @artefact API

The :program:`workshopd` daemon exposes a versioned REST API (v1) over Unix domain sockets
for secure local communication with the CLI.
The API provides endpoints for all workshop operations:

- Project management (:samp:`/v1/projects`)
- Workshop life cycle operations (:samp:`/v1/projects/<ID>/workshops`)
- Workshop execution and control (:samp:`/v1/projects/<ID>/workshops/<NAME>/exec`)
- Interface connection management (:samp:`/v1/connections`)
- Change tracking and monitoring (:samp:`/v1/changes`)
- Warning and error reporting (:samp:`/v1/warnings`)
- Workshop control interface (:samp:`/v1/workshopctl`)


The API implements proper access control
and supports both synchronous and asynchronous operations.
Long-running operations return change IDs that can be used
to track progress and wait for completion.

The API uses JSON for data exchange, with well-defined types
for workshop information, SDK details, and interface connections.
It implements proper validation of input data
and returns structured error responses
with appropriate HTTP status codes.


LXD communication
^^^^^^^^^^^^^^^^^

.. @artefact workshopd

The :program:`workshopd` daemon maintains a persistent connection to LXD
through its Unix domain socket at :file:`/var/snap/lxd/common/lxd/unix.socket`,
managing container operations and user isolation.

For each user, :program:`workshopd` creates dedicated LXD projects
(:samp:`workshop.<USERNAME>` and :samp:`workshop-stash.<USERNAME>`)
that provide complete isolation between workshops
on the same system.

The LXD communication layer handles container life cycle,
storage management, network configuration,
and device pass-through, with proper error handling
for common issues like :program:`workshopd` availability
and resource constraints.

.. note::

   The term 'project' in relation to |ws_markup|
   can be used in two unrelated senses:

   - LXD projects, identified by their names
     (e.g., :samp:`workshop.john`).
     These are created at :program:`workshopd`'s request
     to provide isolation in LXD.
     The names are prefixed with :samp:`workshop.` or :samp:`workshop-stash.`
     followed by the username.

   - Workshop projects, identified by |ws_markup|-assigned IDs.
     These are user-defined directories (e.g., :samp:`my-workshop`)
     used to organize and manage workshops.
     They are referenced in CLI commands and API endpoints.


Control interface
~~~~~~~~~~~~~~~~~

.. @artefact workshopctl

The :program:`workshopctl` tool serves as a secure bridge
between workshop containers and the host system.
It operates with the workshop user's permissions (UID 1000)
and communicates with :program:`workshopd`
through a dedicated Unix domain socket
at :file:`/var/lib/workshop/run/workshop.socket.untrusted`.



Management and isolation
------------------------

.. @artefact workshopd

The :program:`workshopd` daemon manages the complete workshop life cycle,
including storage pool management,
image management,
network isolation,
and project isolation.


Storage pool
~~~~~~~~~~~~

|ws_markup| leverages ZFS for its storage needs, managed via LXD.

Workshop requires a minimum pool size of 5 GiB.

The ZFS pool serves multiple purposes beyond simple container storage,
managing container root file systems,
volumes for SDKs and SDK data persistence,
snapshots for quick workshop updates and rollbacks,
and cached container images.


Images
~~~~~~

|ws_markup| containers are created from base operating system images.
By default, |ws_markup| fetches base images,
such as Ubuntu 20.04, Ubuntu 22.04, or Ubuntu 24.04,
from the official
`Ubuntu cloud image repository <https://cloud-images.ubuntu.com/releases>`_,
using the
`simplestreams <https://canonical-simplestreams.readthedocs-hosted.com/en/latest/>`_
protocol.

|ws_markup| uses specific aliases for these images within LXD,
typically in the format :samp:`workshop-ubuntu@<VERSION>-<ARCH>`
(e.g., :samp:`workshop-ubuntu@24.04-amd64`).
LXD caches these images locally after the first download.


Network
~~~~~~~

|ws_markup| establishes a dedicated network infrastructure
through the :samp:`workshopbr0` bridge network,
providing isolated networking for workshop containers.

This bridge network includes DNS resolution
configured with the :samp:`workshop` domain.



LXD projects
~~~~~~~~~~~~

|ws_markup| implements user isolation through LXD's project system,
automatically creating dedicated projects for each user
following the naming pattern :samp:`workshop.<USERNAME>`.
Each user project includes a corresponding stash project
(:samp:`workshop-stash.<USERNAME>`)
used for temporary storage during workshop rebuild operations.

Note that this LXD project name is different from the workshop project ID:
the LXD project name (:samp:`workshop.<USERNAME>`) provides user-level isolation,
while each workshop project gets its own unique ID for tracking and management.


The workshop launch includes setting up appropriate ID mapping,
ensuring that files created within workshops
maintain correct ownership when accessed from the host system.
This is achieved through LXD's ID mapping feature,
which maps the host user's UID and GID to the workshop user's IDs (1000:1000)
inside the container.


Diagrams
--------

Core installation components:

.. mermaid::
    :alt: Diagram showing the user interacting with Workshop CLI on the host.
          The CLI communicates with the 'workshopd' daemon, also on the host.
          The daemon interacts with the LXD daemon on the host.
          LXD manages a ZFS storage pool ('workshop')
          and pulls base images from an external image server.
          The dependency is shown on the host.
          This does not show 'workshopctl' for simplicity.
    :align: center
    :config: {"theme":"neutral"}

    flowchart LR
        user([User])

        subgraph HOST [Host system]
            direction TB
            cli([CLI])
            daemon(["workshopd"])
            lxd([LXD daemon])

            cli <--> daemon
            daemon <--> lxd

            subgraph ZFS_Store [ZFS storage]
                zfs_pool[("ZFS pool<br>(workshop)")]
            end
        end



        subgraph EXT_IMG [Image server]
            img_server[("cloud-images.ubuntu.com")]
        end

        user <--> cli
        lxd <--> zfs_pool
        lxd <--> img_server


Data flow between components:

.. mermaid::
    :alt: Diagram showing the data flow between Workshop components.
          The user interacts with the CLI, which communicates with :program:`workshopd`.
          workshopd manages LXD operations and handles workshop life cycle.
          LXD manages containers and storage, while the CLI provides user feedback.
          This does not show 'workshopctl' interactions for simplicity.
    :align: center
    :config: {"theme":"neutral"}

    sequenceDiagram
        participant User
        participant CLI as Workshop CLI
        participant Daemon as workshopd
        participant LXD
        participant Storage as ZFS storage

        User->>CLI: Command
        CLI->>Daemon: REST API request
        Daemon->>LXD: Container operation
        LXD->>Storage: Storage operation
        Storage-->>LXD: Operation result
        LXD-->>Daemon: Operation status
        Daemon-->>CLI: API response
        CLI-->>User: Command result
