:slug: tutorial
.. _tutorial:

Tutorial
========

This is a practical introduction
that takes you on a tour
of the essential |ws_markup| activities.

.. @artefact workshop (container)

A :ref:`workshop <exp_workshop>` is an environment
that maps your project to its contained dependencies.
Here, you will practise all the major steps
in the life cycle of a *workshop*,
from :ref:`defining <tut_define>`, :ref:`launching <tut_launch>`
and :ref:`refreshing <tut_refresh>` it
to :ref:`executing commands <tut_exec>`,
:ref:`shelling <tut_shell>` into the workshop
and finally :ref:`removing <tut_remove>` it.
The actions you're about to perform
cover most of your daily needs with |ws_markup|.

If you need a more descriptive overview,
refer to the
:ref:`explanation <exp_index>` section.
For comprehensive details, explore the
:ref:`reference <ref_index>` section.
Finally,
if you're looking for advanced practical steps,
see the
:ref:`how-to guides <how_index>`.


.. _tut_install:

Install |ws_markup|
-------------------

Check the prerequisites,
build and install |ws_markup|,
then ensure it runs.


Prepare LXD
~~~~~~~~~~~

|ws_markup| relies on
`LXD 6.3+ <https://canonical.com/lxd>`_
for low-level operation
and uses its
`API <https://documentation.ubuntu.com/lxd/en/latest/restapi_landing/>`_
to handle individual *workshops*.

Check whether it's configured:

.. code-block:: console

   $ lxc info


If not, `install <https://documentation.ubuntu.com/lxd/en/latest/installing/>`_
and
`initialise <https://documentation.ubuntu.com/lxd/en/latest/howto/initialize/>`_
LXD.

.. tabs::
   .. group-tab:: Using :program:`snap`

      It's available as a snap:

      .. code-block:: console

         $ sudo snap install lxd
         $ sudo lxd init --auto


   .. group-tab:: Other ways

      See the available installation options in
      `LXD documentation
      <https://documentation.ubuntu.com/lxd/en/latest/installing/>`_.


Next, ensure the
`LXD daemon
<https://documentation.ubuntu.com/lxd/en/latest/explanation/lxd_lxc/#lxd-daemon>`_
is enabled and running:

.. tabs::
   .. group-tab:: Using :program:`snap`

      .. code-block:: console

         $ sudo snap start --enable lxd.daemon
         $ snap services lxd.daemon

   .. group-tab:: Other ways

      Refer to
      `LXD documentation
      <https://documentation.ubuntu.com/lxd/en/latest/installing/>`_
      and your distribution's manuals for guidance.


With LXD installed and initialised,
proceed to installing |ws_markup|.


Install
~~~~~~~

Download the latest snap from |ws_markup|'s `Releases`_ page on GitHub
and install it, using the options
`--dangerous <https://snapcraft.io/docs/install-modes>`_
and
`--classic <https://snapcraft.io/docs/install-modes>`_,
for example:

.. code-block:: console

   $ sudo snap install --dangerous --classic ./workshop_0.1.12_amd64.snap


The command installs two main components:

.. @artefact installation
.. @artefact workshopd
.. @artefact workshop (CLI)
.. @artefact API

- The :program:`workshopd` daemon, which exposes a REST API

- The :program:`workshop`
  :ref:`CLI tool <exp_workshop_cli>`,
  which uses this API to command |ws_markup|


After installation, the daemon should start and stop on demand.
Make sure it's enabled:

.. code-block:: console

   $ snap services workshop.workshopd

     Service             Startup  Current   Notes
     workshop.workshopd  enabled  inactive  socket-activated


Run
~~~

Before proceeding, ensure the CLI tool works:

.. @artefact workshop --help

.. code-block:: console

   $ workshop --help


This should display the available commands and usage information.
Now that |ws_markup| is operational,
you're ready to create your first workshop.

.. note::

   If anything went wrong in this section, see this guide:
   :ref:`how_troubleshoot`.


Install shell completions
~~~~~~~~~~~~~~~~~~~~~~~~~

|ws_markup| features shell completion for popular shells
such as :program:`bash`, :program:`zsh` and :program:`fish`.
Bash completion is configured automatically;
you can :ref:`install completion for other shells <ref_workshop_cli_completion>`
manually.

After that, press the :kbd:`Tab` key while typing a command
to quickly substitute suitable subcommands, flags and arguments.


.. _tut_define_launch:

Launch a workshop
-----------------

Now you'll learn how to define, launch, start and stop a workshop.


.. _tut_define:

Define
~~~~~~

First, you need to define a workshop.
A :ref:`definition <exp_workshop_definition>` lists the components of a workshop
to be instantiated at launch
and is stored in your project directory.

.. @artefact sdkcraft (CLI)
.. @artefact SDK
.. @artefact SDK publisher
.. @artefact SDK Store

We'll be focusing on :ref:`SDKs <exp_sdk>`,
which are the basic units of a workshop's functionality.
They are :ref:`built with SDKcraft <how_sdkcraft>` by SDK publishers
to be published on the SDK Store.
At run-time, |ws_markup| pulls and installs them,
providing the dependencies and packages required for your work,
while keeping the SDKs themselves isolated and manageable.

Here, we'll use the sample :samp:`go` SDK,
which was already defined, built and published in the SDK Store
by the |ws_markup| team.

.. @artefact project

Create a
:ref:`project directory <exp_projects>`
named :file:`hello-workshop`:

.. code-block:: console

   $ mkdir ./hello-workshop
   $ cd ./hello-workshop


Everything you plan to build using your workshop goes here:
your source code, custom assets, and so on.
In this tutorial, we'll be building some Go code.

.. @artefact workshop definition

In the project directory,
create a workshop definition named :file:`workshop.yaml`:

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 4

   name: dev
   base: ubuntu@22.04
   sdks:
     - name: go
       channel: jammy/stable


Here, the SDK is referenced as :samp:`go`,
and the specific version to retrieve from the SDK Store
comes from the :samp:`jammy/stable` channel.

.. tip::

   This tutorial relies on a number of Go samples for demonstration purposes.
   however, this doesn't imply that |ws_markup| is focused solely on Go;
   quite the contrary, it's envisioned as language- and framework-agnostic.


To confirm that |ws_markup| sees the definition,
:ref:`list <ref_workshop_list>` the workshops
in the project directory:

.. @artefact workshop list

.. code-block:: console

   $ workshop list

     Project                Workshop   Status  Notes
     ~/hello-workshop       dev        Off     -


As the output suggests, your newly defined workshop is *Off*,
so it needs to be launched.


.. _tut_launch:

Launch
~~~~~~

To get a workshop ready for use, you :ref:`launch <ref_workshop_launch>` it:

.. @artefact workshop launch

.. code-block:: console

   $ workshop launch


Now, the workshop is *Ready*;
you can start using it to build, debug and run your code.

.. note::

   If anything went wrong in this section, see this guide:
   :ref:`how_debug_issues_workshops`.


After launching, check the run-time :ref:`info <ref_workshop_info>`
to see what went into your workshop:

.. @artefact workshop info

.. code-block:: console

   $ workshop info

     name:     dev
     base:     ubuntu@22.04
     project:  /home/user/hello-workshop
     status:   ready
     notes:    -
     sdks:
       go:
         tracking:   jammy/stable
         installed:  1.23.0  2024-08-15  (51)
         mounts:
           mod-cache:
             host-source:      .../6b79e889/dev/mount/go/mod-cache
             workshop-target:  /home/workshop/go/pkg/mod


The output looks like the :ref:`definition <tut_define>`
with extra details such as the :ref:`mounts <tut_interfaces>`;
you can ignore these for now.

.. @artefact workshop .lock

After launch, |ws_markup| starts tracking the project directory.
The workshop stays operational with no extra steps on your part
by using a hidden :file:`.lock` file that must remain in the project directory
and not be copied or stored externally, e.g. in a repository.

To briefly glimpse the steps of the latest change,
use :command:`workshop tasks` without a change ID:

.. @artefact workshop tasks

.. code-block:: console

   $ workshop tasks


For a historical view,
check out the list of recent :ref:`changes <ref_workshop_changes>`
to see how |ws_markup| keeps track of the project directory:

.. @artefact workshop changes

.. code-block:: console

   $ workshop changes

     ID  Status  Spawn               Ready               Summary
     34  Done    today at 11:32 GMT  today at 11:33 GMT  Launch "dev" workshop


To find out what launching a workshop implies,
pass the ID of the change to the :ref:`tasks <ref_workshop_tasks>` command:

.. code-block:: console

   $ workshop tasks 34

     ID   Status  Spawn               Ready               Summary
     133  Done    today at 11:32 GMT  today at 11:32 GMT  Create new "dev" workshop
     134  Done    today at 11:32 GMT  today at 11:32 GMT  Mount project directory "hello-workshop"
     135  Done    today at 11:32 GMT  today at 11:32 GMT  Start "dev" workshop
     136  Done    today at 11:32 GMT  today at 11:32 GMT  Retrieve "go" SDK from channel "latest/stable"
     137  Done    today at 11:32 GMT  today at 11:32 GMT  Install "go" SDK
     138  Done    today at 11:32 GMT  today at 11:33 GMT  Link "go" SDK
     139  Done    today at 11:33 GMT  today at 11:33 GMT  Run hook "setup-base" for "go" SDK
     140  Done    today at 11:33 GMT  today at 11:33 GMT  Auto-connect interfaces of "go" SDK


Here, the following happens:

- The :ref:`project directory <tut_define>`
  is mounted inside the workshop
  (remember that it's a container)
  as :file:`/project/`.

- The workshop is *started*, or brought online.

- The :samp:`go` SDK from the definition is retrieved,
  installed and set up inside the workshop.

- The :ref:`interfaces <exp_interfaces>` of the SDK are connected.


You only need to launch a workshop once after defining it;
for any subsequent changes, you can do a :ref:`refresh <tut_refresh>`.
Otherwise, the workshop is just a fancy container
that can be started and stopped.


Start and stop
~~~~~~~~~~~~~~

The workshop starts automatically at launch,
but you can also stop and restart it at will.

Suppose you want to free up some resources,
so you :ref:`stop <ref_workshop_stop>` the workshop:

.. @artefact workshop stop

.. code-block:: console

   $ workshop stop

This changes the status of the workshop to *Stopped*.

To make it *Ready* again, :ref:`start <ref_workshop_start>` the workshop:

.. @artefact workshop start

.. code-block:: console

   $ workshop start


Both commands work gracefully,
waiting for the workshop to comply:

- :command:`workshop stop` doesn't destroy the workshop,
  unlike :ref:`remove <tut_remove>`

- :command:`workshop start` doesn't build it from scratch,
  unlike :ref:`launch <tut_launch>` or :ref:`refresh <tut_refresh>`


In the next step, you'll refresh an existing workshop.


.. _tut_refresh:

Refresh a workshop
------------------

Sometimes the
:ref:`base <exp_base>`
or the
:ref:`SDKs <exp_sdk>`
listed in your :ref:`workshop definition <tut_define>`
are updated by their publishers.
Alternatively,
you may have changed the definition to switch bases,
add and remove SDKs or toggle their channels.
In either case,
you should :ref:`refresh <ref_workshop_refresh>` the workshop
to apply the updates.

To do so, change the base and the SDK channel in your definition
and refresh the workshop:

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 2,5

   name: dev
   base: ubuntu@24.04
   sdks:
     - name: go
       channel: noble/stable

.. @artefact workshop refresh

.. code-block:: console

   $ workshop refresh


Running :command:`workshop refresh` is similar to a :ref:`launch <tut_launch>`.
However, it ensures the workshop remains operational.
If issues occur, a refresh rolls back to a previous stable condition,
whereas a failed launch has no condition to revert to and just fails.
For help, see this guide: :ref:`how_debug_issues_workshops`.


Now that you can launch, refresh, start and stop a workshop,
let's move on to more practical purposes.


.. _tut_exec:

Execute commands
----------------

When the workshop is *Ready*,
you can run arbitrary commands in it.
In this tutorial, we're building Go code, so let's write some.

In the project directory, save this code as :file:`main.go`:

.. code-block:: go
   :caption: main.go

   package main

   import "fmt"

   func main() {
     fmt.Println("Hello, Workshop")
   }


Next, build it *inside the workshop* using :ref:`exec <ref_workshop_exec>`:

.. @artefact workshop exec

.. code-block:: console

   $ workshop exec dev go build main.go

.. tip::

   Since :samp:`dev` is the only workshop in the project,
   it can be omitted from most :command:`workshop` commands.
   For :ref:`exec <ref_workshop_exec>`,
   a name or a separator (:samp:`--`) is required to avoid ambiguity.
   The above command can also be written as:

   .. code-block:: console

      $ workshop exec -- go build main.go


This uses the Go version installed by the :samp:`go` SDK.

You can define environment variables for the :command:`go build` command
or separate it from :command:`workshop exec` options for clarity:

.. code-block:: console

   $ workshop exec --env GO111MODULE=off dev -- go build -x

The binary, built within the workshop environment,
is now available in the project directory.

**This is the single most important part of the tutorial**;
your deliverables, however complex they are, end up on the host system,
while the tool chain is transparently confined and managed by |ws_markup|.

Next, we'll explore the remaining aspects of your daily workshop usage.


.. _tut_shell:

Interactive shell
~~~~~~~~~~~~~~~~~

Besides running individual commands,
you can open an interactive :ref:`shell <ref_workshop_shell>`
if you need to perform multiple operations within a session.
|ws_markup| runs the login shell
for the default non-privileged user,
who's also named :samp:`workshop`:

.. @artefact workshop shell

.. code-block:: console

   $ workshop shell
   workshop@dev-6b79e889:~$ pwd

     /home/workshop

   workshop@dev-6b79e889:~$ uname -a
   workshop@dev-6b79e889:~$ exit


.. _tut_project_updates:

Project directory updates
~~~~~~~~~~~~~~~~~~~~~~~~~

Remember that the project directory is mounted as :file:`/project/`
when the workshop is launched;
any changes to :file:`/project/` from inside the workshop
are visible in the project directory, and vice versa:

.. code-block:: console

   $ touch created_outside.txt
   $ workshop exec -- ls /project/
   $ workshop exec -- touch /project/created_inside.txt
   $ ls


This isn't the only way the host interacts with the workshop;
let's dive into how interfaces operate.


.. _tut_interfaces:

Work with interfaces
--------------------

.. @artefact interface
.. @artefact system SDK

For security and control,
|ws_markup| provides various host system capabilities (camera, GPU, and so forth)
to the workshop through the :ref:`interface mechanism <exp_interfaces>`,
using :ref:`plugs and slots <exp_plugs_slots>`.

SDKs use interfaces to interact in an organised manner,
exposing the resources they provide via slots and consuming them via plugs;
the layout of these plugs and slots is defined by the SDK publishers.
Host system resources are similarly exposed to the |ws_markup| ecosystem
through :ref:`system SDK <exp_system_sdk>` slots.

To list the connected interfaces,
use :ref:`connections <ref_workshop_connections>`:

.. @artefact workshop connections

.. code-block:: console

   $ workshop connections

     Interface  Plug              Slot              Notes
     mount      dev/go:mod-cache  dev/system:mount  -


This lists a :ref:`mount interface <exp_mount_interface>` plug
named :samp:`dev/go:mod-cache`.
As seen in the :command:`workshop info` output,
it was automatically connected at :ref:`launch <tut_launch>`
to the :samp:`dev/system:mount` slot,
indicated by the ellipsis in the :samp:`host-source` path
and abbreviated here as :samp:`:mount` by convention.

Some interfaces are auto-connected, while some are not;
this usually depends on their purpose.

In any case, you can :ref:`connect <ref_workshop_connect>`
and :ref:`disconnect <ref_workshop_disconnect>` interfaces at will:

.. @artefact workshop connect
.. @artefact workshop disconnect

.. code-block:: console

   $ workshop disconnect dev/go:mod-cache
   $ workshop connect dev/go:mod-cache :mount


You can :ref:`remount <ref_workshop_remount>` a mount interface plug
to a new location on the host:

.. @artefact workshop remount

.. code-block:: console
   :emphasize-lines: 14

   $ workshop remount dev/go:mod-cache ~/mod/
   $ workshop info

     name:     dev
     base:     ubuntu@24.04
     project:  /home/user/hello-workshop
     status:   ready
     notes:    -
     sdks:
       go:
         tracking:   noble/stable
         installed:  1.23.3  2024-11-09  (54)
         mounts:
           mod-cache:
             host-source:      /home/user/mod
             workshop-target:  /home/workshop/go/pkg/mod


This makes :file:`/home/user/mod/` on the host
act as the Go modules cache for the workshop.

Lastly, you can add plugs and slots to the SDKs in the workshop definition,
allowing you to tailor the initial plug and slot layout to your requirements.
For instance, you could use the :ref:`tunnel interface <exp_tunnel_interface>`
with the system SDK to connect to a server running in the workshop.

.. @artefact tunnel interface

For a quick demo, let's install `Caddy <https://caddyserver.com/>`_
to serve files over HTTP:

.. code-block:: console

   $ workshop exec -- go install github.com/caddyserver/caddy/v2/cmd/caddy@latest
   $ cat <<EOF > Caddyfile
   :8080 {
           file_server
   }
   EOF
   $ echo 'Hello, Workshop!' > index.html


This installs Caddy inside the workshop under :file:`~/go/bin/`
in the :samp:`workshop` user's home directory,
configures it to run as a file server at port 8080
and creates an index file.

.. note::

   We added the index file to the project directory on the host;
   however, the server will be able to access it
   because the project directory is mounted inside the workshop.


To configure the tunnel interface,
add the following lines to the definition:

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 6-14

   name: dev
   base: ubuntu@24.04
   sdks:
     - name: go
       channel: noble/stable
       slots:
         caddy:
           interface: tunnel
           endpoint: localhost:8080
     - name: system
       plugs:
         caddy:
           interface: tunnel
           endpoint: localhost:8080


First, this defines a :samp:`go:caddy` slot under the :samp:`go` SDK,
used to expose the server running inside the workshop.
This slot isn't part of the SDK by default;
it's defined for this workshop only,
so other instances of the :samp:`go` SDK in other workshops won't have it.

Additionally, this adds a plug named :samp:`system:caddy`
to indicate that the system SDK in this workshop
can connect to a tunnel interface slot and expose it in the host system.

Refresh the workshop to enable the tunnel;
|ws_markup| matches the plug to the slot using their names,
then validates and enables the connection.
Check the result using :command:`workshop info`:

.. code-block:: console

   $ workshop refresh
   $ workshop info

     ...
     sdks:
       system:
         tunnels:
           server:
             from:  127.0.0.1:8080/tcp
             to:    127.0.0.1:8080/tcp
     ...

Then start the server at port 8080 (the slot):

.. code-block:: console

   $ workshop exec -- caddy start


By default,
:command:`exec` uses the :file:`/project/` directory in the workshop
as the current working directory
so Caddy will serve the files in it.
Finally, test the server on the host at port 8080 (the plug):

.. code-block:: console

   $ curl localhost:8080

     Hello, Workshop!


.. _tut_sketch:

Sketch an SDK (optional)
------------------------

Another way to customise a workshop in-place is called *sketching*.
This process grafts a :ref:`special SDK <exp_sketch_sdk>` onto the workshop,
so you can run a quick local experiment
and circumvent the usual SDK Store publishing workflow.

Sketching an SDK involves finer details
covered in the :ref:`how-to guide <how_sketch>`.
You'll also need a basic understanding of SDK concepts
such as :ref:`plugs, slots <exp_plugs_slots>` and :ref:`hooks <exp_hooks>`
to use them effectively with :command:`workshop sketch-sdk`.


.. _tut_remove:

Remove a workshop
-----------------

We're at the end of our tutorial;
the only thing left is the cleanup.

If you no longer need your workshop,
:ref:`remove <ref_workshop_remove>` it:

.. @artefact workshop remove

.. code-block:: console

   $ workshop remove


This doesn't affect the files in the project directory,
including the workshop definition,
or any other content that was stored outside the workshop,
e.g. via the :ref:`mount interface <tut_interfaces>`.

.. important::

   Don't delete the project directory without first removing the workshop.
   Otherwise, you'll need to manually delete the orphaned workshops;
   for help, see this guide: :ref:`how_troubleshoot_lxc`.


Next steps
----------

This was the last step in the tutorial;
you are now familiar with the essential operations provided by |ws_markup|
and have had your first taste of what it can do for you.

- If you wish to try building and publishing a full-fledged SDK,
  continue to the |sdk_markup| :ref:`how-to guide <how_sdkcraft>`;
  the :ref:`ROS 2 case study <how_ros2>`
  describes the entire process of building an SDK and using it in |ws_markup|
  in extra detail.

- For advanced scenarios and use cases,
  see other :ref:`how-to guides <how_index>`.

- To know more about workshops in general,
  proceed to :ref:`explanation <exp_index>`
  and :ref:`reference <ref_index>` sections.
