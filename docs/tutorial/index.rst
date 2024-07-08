:slug: tutorial
.. _tutorial:

Tutorial
========

This is a practical introduction
that takes you on a tour
of the essential |project_markup| activities.

You will practice all the major steps
in the life cycle of a *workshop*,
from :ref:`defining <tut_define>`, :ref:`launching <tut_launch>`
and :ref:`refreshing <tut_refresh>` it
to :ref:`executing commands <tut_exec>`,
:ref:`shelling <tut_shell>` into the workshop
and finally :ref:`deleting <tut_remove>` it.
The actions you're about to perform
cover most of your daily needs with |project_markup|.

If you need a more descriptive overview,
refer to the
:ref:`explanation <exp_index>`.
For comprehensive details, explore the
:ref:`reference <ref_index>`.
Finally,
if you're looking for advanced practical steps,
see the
:ref:`how-to guides <howto_index>`.


Install |project_markup|
------------------------

Check the prerequisites,
build and install |project_markup|,
then ensure it runs.


Check prerequisites
~~~~~~~~~~~~~~~~~~~

|project_markup| relies on
`LXD 5.21+ <https://canonical.com/lxd>`_
for low-level operation
and uses its
`API <https://documentation.ubuntu.com/lxd/en/latest/restapi_landing/>`_
to handle individual *workshops*.

.. note::

   This means you can use regular :command:`lxc` commands
   to monitor |project_markup| activity, for example:

   .. code-block:: console

      $ lxc list --all-projects


   Also, this means you can use |project_markup| anywhere LXD runs, including
   `Ubuntu WSL
   <https://canonical-ubuntu-wsl.readthedocs-hosted.com/en/latest/>`_.

   However, it's not recommended to rely on this implementation detail.


First, install and
`initialise <https://documentation.ubuntu.com/lxd/en/latest/howto/initialize/>`_
LXD.
It's available as a snap:

.. tabs::
   .. group-tab:: Using :program:`snap`

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


Install
~~~~~~~

Build the :program:`workshop` snap
from the |project_markup| source code on
`GitHub`_:

.. code-block:: console

   $ git clone git@github.com:canonical/workshop.git  # or git clone https://github.com/canonical/workshop.git
   $ cd workshop
   $ sudo snap install snapcraft --classic
   $ snapcraft clean && snapcraft

.. tip::

   In case of :program:`lxd`-related issues with :program:`snapcraft`,
   ensure you're a member of the :samp:`lxd` group:
   
   .. code-block:: console
      
      $ id -nG <USERNAME>
      $ sudo adduser <USERNAME> lxd

Install the resulting :file:`.snap` file,
for example:

.. code-block:: console

   $ sudo snap install --dangerous --classic ./workshop_0.1.0_amd64.snap


Run
~~~

The snap installs two main components:

- The :program:`workshopd` daemon, which exposes a REST API

- The :program:`workshop`
  :ref:`CLI tool <exp_workshop_cli>`,
  which uses this API to command |project_markup|


The daemon starts automatically after installation,
but you run the CLI tool manually:

.. code-block:: console

   $ workshop --help


Launch a workshop
-----------------

Once you have installed |project_markup|,
use it to define, launch, start and stop your first
:ref:`workshop <exp_workshop>`.


.. _tut_define:

Define
~~~~~~

#. Create a
   :ref:`project directory <exp_projects>`
   named :file:`hello-workshop`:

   .. code-block:: console

      $ mkdir ~/hello-workshop
      $ cd ~/hello-workshop


#. In the project directory,
   create a
   :ref:`workshop definition <exp_workshop_def>`
   named :file:`.workshop.golang.yaml`:

   .. code-block:: yaml
      :caption: .workshop.golang.yaml

      name: golang
      base: ubuntu@22.04
      sdks:
        go:
          channel: latest/stable


#. To ensure |project_markup| sees the definition,
   :ref:`list <ref_workshop_list>` the workshops
   in the project directory:

   .. code-block:: console

      $ workshop list

        Project                Workshop   Status  Notes
        ~/hello-workshop       golang     Off     -


   A newly defined workshop is *Off*,
   so it needs to be initialised, or *launched*.


.. _tut_launch:

Launch
~~~~~~

To prepare a workshop for action,
you :ref:`launch <ref_workshop_launch>` it:

.. code-block:: console

   $ workshop launch golang


Now, the workshop is *Ready*
to build, debug and run code.

.. note::

   If something goes wrong at this point, see the
   :ref:`debugging guide <how_debug_issues_workshops>`.

After launching, you can see the run-time :ref:`info <ref_workshop_info>`:

.. code-block:: console

   $ workshop info golang

     name:     golang
     base:     ubuntu@22.04
     project:  /home/user/hello-workshop
     status:   ready
     notes:    -
     content:
       go:
         channel:  latest/stable
         mounts:
           mod-cache:
             host:      .../a4361706/content/golang_go_mod-cache.sdk
             workshop:  /home/workshop/go/pkg/mod


The output is similar to the :ref:`definition <tut_define>`
but includes extra details
such as the :ref:`content interface <tut_interfaces>` mounts.

Note that |project_markup| tracks the project directory after launch.
Try moving it temporarily and re-run :command:`list`:

.. code-block:: console

   $ cd ..
   $ mv hello-workshop hi-workshop
   $ workshop list --global

     Project                Workshop   Status  Notes
     ~/hi-workshop          golang     Ready   -

   $ mv hi-workshop hello-workshop
   $ cd hello-workshop


This means that the workshop stays operational without extra steps on your part.

.. note::

   This is achieved by using a hidden :file:`.lock` file;
   it must remain in the project directory
   and must not be copied or stored externally, e.g. in a repository.


Check out the recent :ref:`changes <ref_workshop_changes>`
to see how |project_markup| keeps track of its environment:

.. code-block:: console

   $ workshop changes

     ID  Status  Spawn               Ready               Summary
     34  Done    today at 11:32 GMT  today at 11:33 GMT  Launch "golang" workshop

To find out what goes into launching a workshop,
pass the ID of the change to the
:ref:`tasks <ref_workshop_tasks>`
command:

.. code-block:: console

   $ workshop tasks 34

     ID   Status  Spawn               Ready               Summary
     133  Done    today at 11:32 GMT  today at 11:32 GMT  Create new "golang" workshop
     134  Done    today at 11:32 GMT  today at 11:32 GMT  Mount project directory "hello-workshop"
     135  Done    today at 11:32 GMT  today at 11:32 GMT  Start "golang" workshop
     136  Done    today at 11:32 GMT  today at 11:32 GMT  Retrieve "go" SDK from channel "latest/stable"
     137  Done    today at 11:32 GMT  today at 11:32 GMT  Install "go" SDK
     138  Done    today at 11:32 GMT  today at 11:33 GMT  Link "go" SDK
     139  Done    today at 11:33 GMT  today at 11:33 GMT  Run hook "setup-base" for "go" SDK
     140  Done    today at 11:33 GMT  today at 11:33 GMT  Auto-connect interfaces of "go" SDK


Here, the project directory is mounted to the workshop as :file:`/project/`;
the workshop is *started*, or brought online;
then the :samp:`go` SDK,
which was referenced in the :ref:`definition <tut_define>`,
is retrieved, installed and set up inside the workshop;
then the SDK is connected to the host system
via :ref:`interfaces <exp_interfaces>`.

Finally, mind that you only need to launch a workshop once
after defining it.


Start and stop
~~~~~~~~~~~~~~

If you're finished with the workshop for now,
:ref:`stop <ref_workshop_stop>` it to save resources:

.. code-block:: console

   $ workshop stop golang

This changes the status of the workshop to *Stopped*.

To make it *Ready* again, :ref:`start <ref_workshop_start>` the workshop:

.. code-block:: console

   $ workshop start golang


Both commands work gracefully,
waiting for the workshop to comply;
:command:`stop` doesn't destroy a workshop
(unlike :ref:`remove <tut_remove>`),
and :command:`start` doesn't build it from scratch
(unlike :ref:`launch <tut_launch>` or :ref:`refresh <tut_refresh>`).


.. _tut_refresh:

Refresh a workshop
------------------

Sometimes the
:ref:`base <exp_base>`
or the
:ref:`SDKs <exp_sdk>`
in your workshop
are updated by their publishers.
Alternatively,
you may have changed the :ref:`definition <exp_workshop_def>`
to switch bases, add and remove SDKs or toggle their channels.
In either case,
:ref:`refresh <ref_workshop_refresh>` the workshop
to apply the updates.

Change the base in your :ref:`definition <tut_define>`
and refresh the workshop:

.. code-block:: yaml
   :caption: .workshop.golang.yaml
   :emphasize-lines: 2

   name: golang
   base: ubuntu@20.04
   sdks:
     go:
       channel: latest/stable


.. code-block:: console

   $ workshop refresh golang


In general, :command:`refresh` is similar to a :ref:`launch <tut_launch>`.
However, its default priority is to keep the workshop operational;
if problems arise, it rolls back.
For more details, see the
:ref:`debugging guide <how_debug_issues_workshops>`.


.. _tut_exec:

Execute commands
----------------

When the workshop is *Ready* after the refresh,
you can :ref:`exec <ref_workshop_exec>` arbitrary commands in it.

Save this Go code in the project directory to build it inside the workshop:

.. code-block:: go
   :caption: main.go

   package main

   import "fmt"

   func main() {
     fmt.Println("Hello, Workshop")
   }


.. code-block:: console

   $ workshop exec golang go build main.go


You can define environment variables for the command,
or separate the command from :command:`exec` options:

.. code-block:: console

   $ workshop exec golang --env GO111MODULE=off -- go build -x


.. _tut_shell:

Interactive shell
~~~~~~~~~~~~~~~~~

Instead of running individual commands,
you can open an interactive :ref:`shell <ref_workshop_shell>`:

.. code-block:: console

   $ workshop shell golang
   workshop@golang-6b79e889:~$ pwd

     /home/workshop

   workshop@golang-6b79e889:~$ uname -a


|project_markup| runs the login shell
for the default non-privileged user,
also named :samp:`workshop`.


Project directory updates
~~~~~~~~~~~~~~~~~~~~~~~~~

Any changes made in :file:`/project/` inside the workshop
are visible in the project directory, and vice versa:

.. code-block:: console

   $ touch outside_workshop.txt
   $ workshop exec golang -- bash -c "ls -l"
   $ workshop exec golang -- touch inside_workshop.txt
   $ ls -l


.. _tut_interfaces:

Work with interfaces
--------------------

For security and control,
|project_markup| exposes various host system capabilities to the workshop
by connecting it to the appropriate :ref:`interfaces <exp_interfaces>`.
To list the connected interfaces,
use :ref:`connections <ref_workshop_connections>`:

.. code-block:: console

   $ workshop connections

     Interface  Plug                 Slot      Notes
     content    golang/go:mod-cache  :content  -


This is the :ref:`content interface <exp_content_interface>`
you've seen :ref:`earlier <tut_launch>`
in the output from :command:`workshop info`.

Some interfaces are auto-connected, while some are not;
this usually depends on their purpose.
In any case, you can :ref:`connect <ref_workshop_connect>`
and :ref:`disconnect <ref_workshop_disconnect>` interfaces at will:

.. code-block:: console

   $ workshop disconnect golang/go:mod-cache
   $ workshop connect golang/go:mod-cache :content


You can :ref:`remount <ref_workshop_remount>` a content interface plug
to a new location on the host:

.. code-block:: console
   :emphasize-lines: 14

   $ workshop remount golang/go:mod-cache ~/new-location/
   $ workshop info golang

     name:     golang
     base:     ubuntu@20.04
     project:  /home/user/hello-workshop
     status:   ready
     notes:    -
     content:
       go:
         channel:  latest/stable
         mounts:
           mod-cache:
             host:      /home/user/new-location
             workshop:  /home/workshop/go/pkg/mod


.. _tut_remove:

Remove a workshop
-----------------

If you're no longer using your workshop,
:ref:`remove <ref_workshop_remove>` it:

.. code-block:: console

   $ workshop remove golang


This doesn't affect the files in the project directory,
including the workshop definition,
or any other content that was stored outside the workshop,
e.g. via the :ref:`content interface <exp_content_interface>`.

.. important::

   Don't delete the project directory without first removing the workshop.


This was the last step in the tutorial;
you are now familiar with the essential operations provided by |project_markup|
and have had your first taste of what it can do for you.
