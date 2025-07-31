.. _tut_get_started:

.. meta::
   :description: Practical introduction to workshops, guiding users through
                 defining, launching, and refreshing workshops, and executing commands in workshops.

Get started with workshops
==========================

This is the first section of the :ref:`four-part series <tut_index>`;
a practical introduction
that takes you on a tour
of the essential |ws_markup| activities.

.. @artefact workshop (container)

A workshop is a development environment running in a container,
mapping your project to its contained dependencies.
Here, you will practice all the major steps
in the life cycle of a *workshop*,
from :ref:`defining <tut_define>`, :ref:`launching <tut_launch>`,
and :ref:`refreshing <tut_refresh>` it
to :ref:`executing commands <tut_exec>`,
:ref:`shelling <tut_shell>` into the workshop,
and finally :ref:`removing <tut_remove>` it.
The actions you're about to perform
cover most of your daily needs with |ws_markup|.


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
`API <https://documentation.ubuntu.com/lxd/latest/restapi_landing/>`_
to handle individual *workshops*.
Check whether it's properly configured:

.. code-block:: console

   $ lxc info | grep 'server_version:'

     server_version: "6.3"


If the command displays an older version
or returns an error indicating LXD is missing,
install a recent LXD version with :program:`snap`.
To install it from scratch:

.. code-block:: console

   $ sudo snap install lxd --channel=6/stable


To refresh an existing installation:

.. code-block:: console

   $ sudo snap refresh lxd --channel=6/stable


.. note::

   For other ways to install LXD,
   see the available installation options in
   `LXD documentation
   <https://documentation.ubuntu.com/lxd/latest/installing/>`_.
   Also, you need to ensure the
   `LXD daemon
   <https://documentation.ubuntu.com/lxd/latest/explanation/lxd_lxc/#lxd-daemon>`_
   is enabled and running.
   Again, refer to LXD documentation
   and your distribution's manuals for guidance.


With LXD properly installed and started,
proceed to installing |ws_markup|.


Install
~~~~~~~

.. @artefact installation
.. @artefact workshopd
.. @artefact workshop (CLI)

Download the latest snap from |ws_markup|'s "Releases" page on GitHub:
:literalref:`https://github.com/canonical/workshop/releases/`


Browse to the download directory and install the snap using the options
`--dangerous <https://snapcraft.io/docs/install-modes>`_
and
`--classic <https://snapcraft.io/docs/install-modes>`_:

.. code-block:: console

   $ sudo snap install --dangerous --classic ./workshop_0.1.20_amd64.snap


Shell integration (optional)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~

|ws_markup| features shell completion for popular shells
such as :program:`bash`, :program:`zsh`, and :program:`fish`.

Bash completion is configured automatically;
manual setup instructions are available for all shells:

.. code-block:: console

   $ workshop completion bash -h
   $ workshop completion fish -h
   $ workshop completion zsh -h


With completion enabled, you can press the :kbd:`Tab` key while typing a command
to quickly substitute suitable subcommands, flags, and arguments.


.. _tut_define_launch:

Launch a workshop
-----------------

Now you'll learn how to define, launch, start and stop a workshop.


.. _tut_define:

Define, add SDKs
~~~~~~~~~~~~~~~~

First, you need to define a workshop.
A definition lists the components of a workshop
to be instantiated at launch
and is stored in your project directory.

.. @artefact sdkcraft (CLI)
.. @artefact SDK
.. @artefact SDK publisher
.. @artefact SDK Store

A definition can list many moving parts;
perhaps, the most important are SDKs,
which are basic, pre-defined units of a workshop's functionality.

You reference SDKs from your workshop definition
to specify what you want to include in your workshop.
At run-time, |ws_markup| pulls and installs them,
providing the dependencies and packages required for your work,
while keeping the SDKs themselves isolated and manageable.

For demonstration purposes, assume we want to work with AI models using Ollama.
To do this, let's use the :samp:`ollama` SDK,
which provides a local AI model server.

.. note::

   The tutorial uses Ollama for demonstration purposes only.
   This doesn't imply that |ws_markup| is intended solely for AI;
   quite the contrary, it's envisioned as language-neutral and framework-agnostic.


.. @artefact project

For the project directory, create a new Python repository:

.. code-block:: console

   $ mkdir ollama-python-project
   $ cd ollama-python-project
   $ git init


Everything you handle with your workshop goes here:
your Python code, custom assets, and so on.

.. @artefact workshop definition

In the project directory,
create a workshop definition named :file:`workshop.yaml`:

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 4,5

   name: dev
   base: ubuntu@24.04
   sdks:
     - name: ollama
       channel: 24.04/edge


Here, the SDK is referenced as :samp:`ollama`,
and the specific version to retrieve from the SDK Store
comes from the :samp:`24.04/edge` channel.

To confirm that |ws_markup| sees the definition,
list the workshops in the project directory:

.. @artefact workshop list

.. code-block:: console

   $ workshop list

     Project                  Workshop  Status  Notes
     ~/ollama-python-project  dev       Off     -


As the output suggests, your newly defined workshop is *Off*,
so it needs to be launched.

.. note::

   For a detailed explanation of the workshop status values,
   see the :ref:`exp_workshop_status` section.


.. _tut_launch:

Launch, start, and stop
~~~~~~~~~~~~~~~~~~~~~~~

To get a workshop ready for use, you launch it:

.. @artefact workshop launch

.. code-block:: console

   $ workshop launch

     "dev" launched


Once the workshop is launched,
you can start using it to build, debug, and run your code.

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
     base:     ubuntu@24.04
     project:  /home/user/ollama-python-project
     status:   ready
     notes:    -
     sdks:
       system:
         installed:  (1)
       ollama:
         tracking:   24.04/edge
         installed:  0.9.6  2025-07-21  (516)
         mounts:
           models:
             host-source:      .../6b79e889/dev/mount/ollama/models
             workshop-target:  /home/workshop/.ollama/models


The output looks like the :ref:`definition <tut_define>`
with extra details such as the :ref:`mounts <tut_interfaces>`;
ignore these for now.

.. @artefact workshop .lock

After launch, |ws_markup| starts tracking the project directory.
The workshop stays operational with no extra steps on your part
by using a hidden :file:`.lock` file that must remain in the project directory
and not be copied or stored externally, e.g., in a repository.

.. note::

   Consider adding the :file:`.lock` file
   to your :file:`.gitignore` or similar ignore files:

   .. code-block:: console

      $ echo ".workshop.lock" >> .gitignore


   In contrast, the definition and the :file:`.workshop/` directory
   are *meant* to be stored in a repository;
   if your :file:`.gitignore` file uses rules
   such as "ignore everything except these files and directories,"
   add them to the list of explicitly tracked items.


You only need to launch a workshop once after defining it;
after any substantial changes to it,
you do a :ref:`refresh <tut_refresh>`.
Otherwise, the workshop is just a fancy container
that can be started and stopped.

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
add and remove SDKs, or toggle their channels.
In either case,
you must refresh the workshop to apply the updates.

For example, change the base and the SDK channel in your definition
and refresh the workshop:

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 2,5

   name: dev
   base: ubuntu@22.04
   sdks:
     - name: ollama
       channel: 22.04/edge

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
In this tutorial, we're working with Ollama AI models,
and we've already created a Python project directory
to serve as our workspace.

.. note::

   |ws_markup| also integrates with modern IDEs.
   For instance, see these guides:
   :ref:`how_vscode_run_in_browser`, :ref:`how_vscode_connect_remote`.


First, let's download a simple AI model *inside the workshop*
using the :command:`workshop exec` command.
We'll use the :samp:`tinyllama` model, which is small and quick to download:

.. @artefact workshop exec

.. code-block:: console

   $ workshop exec dev -- ollama run tinyllama


This downloads and then runs the :samp:`tinyllama` model.
The model will be stored in the mounted :file:`models/` directory,
so it persists between workshop refreshes.

You can also list the available models:

.. code-block:: console

   $ workshop exec dev -- ollama list

     NAME                ID              SIZE      MODIFIED
     tinyllama:latest    2644915ede35    637 MB    24 seconds ago


Furthermore, your work files and deliverables,
however complex they may be, can reside on the host system,
while the toolchain is transparently confined and managed by |ws_markup|.
This enables you to focus on your project,
switching when needed between language and framework versions or base images.

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
   workshop@dev-6b79e889:/project$ pwd

     /project

   workshop@dev-6b79e889:/project$ lsb_release -a

     ...
     Distributor ID: Ubuntu
     Description:    Ubuntu 22.04.5 LTS
     Release:        22.04
     Codename:       jammy

   workshop@dev-6b79e889:/project$ exit


.. _tut_project_updates:

Project directory updates
~~~~~~~~~~~~~~~~~~~~~~~~~

Remember that the project directory is mounted as :file:`/project/`
when the workshop is launched;
any changes to :file:`/project/` from inside the workshop
are visible in the project directory, and vice versa:

.. code-block:: console

   $ touch created_outside.txt
   $ workshop exec dev -- ls /project/

     ...  created_outside.txt  ...

   $ workshop exec dev -- touch /project/created_inside.txt
   $ ls

     ...  created_inside.txt  created_outside.txt  ...


Next, let's dive into how changes and tasks work
to track your workshop activities.


.. _tut_changes_tasks:

Track changes and tasks
-----------------------

To see how |ws_markup| keeps track of its activities around a project,
check out the recent major operations, or changes,
with :command:`workshop changes`:

.. @artefact workshop changes

.. code-block:: console

   $ workshop changes

     ID  Status  Spawn               Ready               Summary
     33  Done    today at 11:20 GMT  today at 11:20 GMT  Refresh "dev" workshop
     34  Done    today at 11:32 GMT  today at 11:32 GMT  Execute command "sudo"


Changes are enacted atomically to ensure workshops stay operational.
Any change must have all its smaller steps, or tasks, succeed;
otherwise, it will be undone.

To look at the latest change,
run the :command:`workshop tasks` command without an argument.
To find out which tasks went into a certain change,
pass the change ID to the command:

.. @artefact workshop tasks

.. code-block:: console

   $ workshop tasks 33

     Status   Duration  Summary
     Done    2m17.389s  Download "ubuntu@22.04" base image
     Done        113ms  Retrieve "system" SDK
     Done    2m59.777s  Retrieve "ollama" SDK from channel "22.04/edge"
     Done        443ms  Create SDK state storage
     Done        581ms  Run hook "save-state" for "system" SDK
     Done        449ms  Run hook "save-state" for "ollama" SDK
     Done         54ms  Disconnect interfaces of "ollama" SDK
     ...
     Done        528ms  Setup "system" SDK profile



This lists all the tasks and includes logs for some of them;
each task expresses a simple token of logic,
such as running a hook or connecting an interface.


.. _tut_remove:

Remove a workshop
-----------------

The only thing left to cover here is the cleanup.

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
   for help, see this how-to guide section: :ref:`how_troubleshoot_lxc`.


Even if you remove the workshop completely,
you can rebuild it with :command:`workshop launch`;
this may come in handy if you have removed your workshop
using the command above
before proceeding to the other parts of the tutorial.


Next steps
----------

This was the last step in this tutorial section;
you are now familiar with the essential operations provided by |ws_markup|
and have had your first taste of what it can do for you.

Your next step is to learn how to work with interfaces;
proceed to the :ref:`tut_work_with_interfaces` section.
