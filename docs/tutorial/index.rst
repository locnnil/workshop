:slug: tutorial
.. _tutorial:

Tutorial
========

This is a practical introduction
that takes you on a tour
of the essential |ws_markup| activities.

.. @artefact workshop (container)

A workshop is an environment
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
If you're looking for advanced practical steps,
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

.. @artefact installation
.. @artefact workshopd
.. @artefact workshop (CLI)

Install the latest snap from |ws_markup|'s `Releases`_ page,
using the options
`--dangerous <https://snapcraft.io/docs/install-modes>`_
and
`--classic <https://snapcraft.io/docs/install-modes>`_:

.. code-block:: console

   $ sudo snap install --dangerous --classic ./workshop_0.1.14_amd64.snap


Enable shell completion
~~~~~~~~~~~~~~~~~~~~~~~

|ws_markup| features shell completion for popular shells
such as :program:`bash`, :program:`zsh` and :program:`fish`.
Bash completion is configured automatically;
for other shells, check out the manual setup instructions:

.. code-block:: console

   $ workshop completion -h


With completion enabled, you can press the :kbd:`Tab` key while typing a command
to quickly substitute suitable subcommands, flags and arguments.


.. _tut_define_launch:

Launch a workshop
-----------------

Now you'll learn how to define, launch, start and stop a workshop.


.. _tut_define:

Define
~~~~~~

First, you need to define a workshop.
A definition lists the components of a workshop
to be instantiated at launch
and is stored in your project directory.

.. @artefact sdkcraft (CLI)
.. @artefact SDK
.. @artefact SDK publisher
.. @artefact SDK Store

We'll be focusing on SDKs,
which are the basic units of a workshop's functionality.
At run-time, |ws_markup| pulls and installs them,
providing the dependencies and packages required for your work,
while keeping the SDKs themselves isolated and manageable.

.. note::

   SDKs are built and published in the Store using |sdk_markup|.
   For details, see this guide:
   :ref:`how_sdkcraft`.


Here, we'll use the sample :samp:`go` SDK,
which was already defined, built and published in the SDK Store
by the |ws_markup| team.

.. @artefact project

Create a project directory named :file:`hello-workshop`:

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
   However, this doesn't imply that |ws_markup| is focused solely on Go;
   quite the contrary, it's envisioned as language-neutral and framework-agnostic.


To confirm that |ws_markup| sees the definition,
list the workshops in the project directory:

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

To get a workshop ready for use, you launch it:

.. @artefact workshop launch

.. code-block:: console

   $ workshop launch


Now, the workshop is *Ready*;
you can start using it to build, debug and run your code.

.. note::

   If issues arise now or later, see these guides:
   :ref:`how_troubleshoot` and
   :ref:`how_debug_issues_workshops`.


After launching, check the run-time information
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
ignore these for now.

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
check out the list of recent changes
to see how |ws_markup| keeps track of the project directory:

.. @artefact workshop changes

.. code-block:: console

   $ workshop changes

     ID  Status  Spawn               Ready               Summary
     34  Done    today at 11:32 GMT  today at 11:33 GMT  Launch "dev" workshop


To find out what launching a workshop implies,
pass the ID of the change to the :command:`workshop tasks` command:

.. code-block:: console

   $ workshop tasks 34

     ID   Status  Spawn               Ready               Summary
     132  Done    today at 11:32 GMT  today at 11:32 GMT  Retrieve "system" SDK
     133  Done    today at 11:32 GMT  today at 11:32 GMT  Retrieve "go" SDK from channel "latest/stable"
     134  Done    today at 11:32 GMT  today at 11:32 GMT  Create apt cache for "dev"
     135  Done    today at 11:32 GMT  today at 11:32 GMT  Download "ubuntu@22.04" base image
     136  Done    today at 11:32 GMT  today at 11:32 GMT  Create new "dev" workshop
     137  Done    today at 11:32 GMT  today at 11:32 GMT  Mount project directory "/home/user/hello-workshop"
     138  Done    today at 11:32 GMT  today at 11:33 GMT  Start "dev" workshop
     139  Done    today at 11:33 GMT  today at 11:33 GMT  Install "system" SDK
     140  Done    today at 11:33 GMT  today at 11:33 GMT  Link "system" SDK
     141  Done    today at 11:33 GMT  today at 11:33 GMT  Run hook "setup-base" for "system" SDK
     142  Done    today at 11:33 GMT  today at 11:33 GMT  Install "go" SDK
     143  Done    today at 11:33 GMT  today at 11:33 GMT  Link "go" SDK
     144  Done    today at 11:33 GMT  today at 11:34 GMT  Run hook "setup-base" for "go" SDK
     145  Done    today at 11:34 GMT  today at 11:34 GMT  Auto-connect interfaces of "system" SDK
     146  Done    today at 11:34 GMT  today at 11:34 GMT  Auto-connect interfaces of "go" SDK
     147  Done    today at 11:34 GMT  today at 11:34 GMT  Run hook "check-health" for "system" SDK
     148  Done    today at 11:34 GMT  today at 11:34 GMT  Run hook "check-health" for "go" SDK
     149  Done    today at 11:34 GMT  today at 11:34 GMT  Connect "dev/go:mod-cache" to "dev/system:mount"
     150  Done    today at 11:34 GMT  today at 11:34 GMT  Connect "dev/go:bin" to "dev/system:mount"
     151  Done    today at 11:34 GMT  today at 11:34 GMT  Setup "system" SDK profile

     ......................................................................
     Run hook "setup-base" for "go" SDK

     2024-08-25T11:34:53+00:00 INFO go 1.23.0 from Canonical** installed
     2024-08-25T11:34:53+00:00 INFO PATH=/home/workshop/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/snap/bin



Here, the following happens:

- The :samp:`ubuntu@22.04` base image from the definition is retrieved.

- A cache is created for apt packages.

- The :ref:`project directory <tut_define>`
  is mounted inside the workshop
  (remember that it's a container)
  as :file:`/project/`.

- The workshop is *started*, or brought online.

- The :ref:`system SDK <exp_system_sdk>` is installed.

- The :samp:`go` SDK from the definition is retrieved,
  installed and set up inside the workshop.

- The interfaces of the SDKs are connected.


You only need to launch a workshop once after defining it;
for any subsequent changes, you can do a :ref:`refresh <tut_refresh>`.
Otherwise, the workshop is just a fancy container
that can be started and stopped.


Start and stop
~~~~~~~~~~~~~~

The workshop starts automatically at launch,
but you can also stop and restart it at will.

Suppose you want to free up some resources, so you stop the workshop:

.. @artefact workshop stop

.. code-block:: console

   $ workshop stop

This changes the status of the workshop to *Stopped*.

To make it *Ready* again, start the workshop:

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

Sometimes the base or the SDKs
listed in your :ref:`workshop definition <tut_define>`
are updated by their publishers.
Alternatively,
you may have changed the definition to switch bases,
add and remove SDKs or toggle their channels.
In either case,
you should refresh the workshop to apply the updates.

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


Next, build it *inside the workshop* using the :command:`workshop exec` command:

.. @artefact workshop exec

.. code-block:: console

   $ workshop exec dev go build main.go

.. tip::

   Since :samp:`dev` is the only workshop in the project,
   it can be omitted from most :command:`workshop` commands.
   For :command:`workshop exec`,
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
you can open an interactive shell
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
to the workshop through the interface mechanism, using plugs and slots.

SDKs use interfaces to interact in an organised manner,
exposing the resources they provide via slots and consuming them via plugs;
the layout of these plugs and slots is defined by the SDK publishers.
Host system resources are similarly exposed to the |ws_markup| ecosystem
through the so-called *system SDK* slots.

To check out the connected interfaces, list the connections:

.. @artefact workshop connections

.. code-block:: console

   $ workshop connections

     Interface  Plug              Slot              Notes
     mount      dev/go:mod-cache  dev/system:mount  -


This lists a mount interface plug named :samp:`dev/go:mod-cache`.
As seen in the :command:`workshop info` output,
it was automatically connected at :ref:`launch <tut_launch>`
to the :samp:`dev/system:mount` slot,
indicated by the ellipsis in the :samp:`host-source` path.

Some interfaces are auto-connected, while some are not;
this usually depends on their purpose.

In any case, you can connect and disconnect interfaces at will:

.. @artefact workshop connect
.. @artefact workshop disconnect

.. code-block:: console

   $ workshop disconnect dev/go:mod-cache
   $ workshop connect dev/go:mod-cache :mount


You can remount a mount interface plug to a new location on the host:

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
For instance, you could use the tunnel interface
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

Another way to customise a workshop in-place
uses the :command:`workshop sketch-sdk` command
and is called *sketching*.
It effectively grafts a special *sketch SDK* onto the workshop,
so you can run a quick local experiment
and circumvent the usual SDK Store publishing workflow.

Sketching an SDK involves finer details
covered in this guide: :ref:`how_sketch`.
You'll also need a basic understanding of SDK concepts
such as plugs, slots and hooks
to use them effectively.


.. _tut_remove:

Remove a workshop
-----------------

We're at the end of our tutorial;
the only thing left is the cleanup.

If you no longer need your workshop,
remove it:

.. @artefact workshop remove

.. code-block:: console

   $ workshop remove


This doesn't affect the files in the project directory,
including the workshop definition,
or any other content that was stored outside the workshop
(e.g. using the :ref:`mount interface <tut_interfaces>`
with a custom :command:`workshop remount` location;
however, the content in *default* mount locations will be deleted).

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
