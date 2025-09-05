.. _tut_craft_sdks:

.. meta::
   :description: Tutorial on crafting SDKs with SDKcraft, teaching users how to
                 initialize, define, pack, and publish SDKs for sharing and use
                 with the CLI utility.

Craft SDKs with |sdk_markup|
==============================

.. @artefact sdkcraft (CLI)
.. @artefact SDK
.. @artefact interface

This is the fourth section of the :ref:`four-part series <tut_index>`;
you'll learn how to create full-featured SDKs
that can be published and shared with others using |sdk_markup|.
It relies on the knowledge gained in the :ref:`tut_get_started` section,
where you learned how to create and run workshops,
and also builds on the :ref:`tut_sketch_sdks` section,
where you learned how to sketch local SDKs.

Here, you will initialize, define, pack, and publish an :ref:`SDK <exp_sdks>`:
a set of hooks, interfaces, and parts that is bundled into a single package,
suitable for use with |sdk_markup|, the user-oriented CLI utility.
The commands you're about to run
cover most of your daily needs with |sdk_markup|.


Check the prerequisites
-----------------------

|sdk_markup| relies on
`LXD 6.3+ <https://canonical.com/lxd>`_
for low-level operation,
using its
`REST API <https://documentation.ubuntu.com/lxd/latest/restapi_landing/>`_
to craft the SDKs.

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


Install |sdk_markup|
--------------------

Download the latest snap from |sdk_markup|'s
`Releases <https://github.com/canonical/sdkcraft/releases/>`__
page on GitHub.

Install it using the
`--dangerous <https://snapcraft.io/docs/install-modes>`_
and
`--classic <https://snapcraft.io/docs/install-modes>`_
options, for example:

.. code-block:: console

   $ sudo snap install --dangerous --classic ./sdkcraft_0.1.12_amd64.snap


The snap installs the :program:`sdkcraft` CLI tool.
Make sure it runs:

.. code-block:: console

   $ sdkcraft --help


.. _how_sdkcraft_init:

Initialize the SDK
------------------

Once you have installed |sdk_markup|,
use it to initialize, define, and pack your first :ref:`SDK <exp_sdks>`.
Here, we'll build an SDK that installs Ollama
for running large language models in the workshop.

First, create a directory named :file:`ollama/`:

.. code-block:: console

   $ mkdir ollama/


.. @artefact SDK definition

It will contain your :ref:`SDK definition <exp_sdk_definition>`
and other source files.

Next, browse to the SDK directory and initialize it:

.. code-block:: console

   $ cd ollama/
   $ sdkcraft init


This command creates a template definition file
named :file:`sdk.yaml`;
although it's almost empty,
it can already be :ref:`built <how_sdkcraft_build_sdk>`.

However, let's take a few extra steps
to explore what goes into an SDK.


Update metadata
---------------

Update the metadata in :file:`sdk.yaml`,
adjusting its :samp:`name`, :samp:`summary` and :samp:`description`:

.. code-block:: yaml
   :caption: sdk.yaml
   :emphasize-lines: 1,4-6

   name: ollama
   version: "0.9.6"
   summary: Get up and running with large language models
   description: |
     Get up and running with Llama 3.3, DeepSeek-R1, Phi-4, 
     Gemma 3, Mistral Small 3.1 and other large language models.
   license: MIT
   platforms:
     ubuntu@22.04:amd64:
     ubuntu@24.04:amd64:

   parts:
     my-part:
       plugin: nil


.. _how_sdkcraft_parts:

Define parts
------------

|sdk_markup| leverages the :ref:`parts mechanism <exp_sdk_parts>`
to obtain data from different sources, process it in various ways,
and prepare an SDK package for publishing.

In our Ollama SDK, we'll define two parts:
one to download the Ollama binary
and another for the :program:`systemd` service file:

.. code-block:: yaml
   :caption: sdk.yaml

   # ...
   parts:
     ollama:
       plugin: dump
       source: https://github.com/ollama/ollama/releases/download/v0.9.6/ollama-linux-amd64.tgz
       source-type: tar
     user-service:
       plugin: dump
       source: ollama.service
       source-type: file


The :samp:`ollama` part uses the :samp:`dump` plugin
to download and extract the official Ollama binary from GitHub releases.
The :samp:`user-service` part includes a :program:`systemd` service file
that will be used to manage the Ollama daemon.

.. note::

   For in-depth details,
   refer to the `Parts
   <https://canonical-craft-parts.readthedocs-hosted.com/en/latest/common/craft-parts/explanation/parts/>`_
   section in Craft Parts documentation.


The first :samp:`dump` downloads the file automatically.
However, we need to create the :program:`systemd` service file
that was referenced in the :samp:`user-service` part.

In the :file:`ollama/` directory,
create a file named :file:`ollama.service`:

.. code-block:: ini
   :caption: ollama.service

   [Unit]
   Description=Ollama Service
   After=network-online.target

   [Service]
   ExecStart=/bin/bash -lc "ollama serve"
   Restart=always
   RestartSec=3

   [Install]
   WantedBy=multi-user.target


This defines how the Ollama daemon should run:

- :samp:`ExecStart` starts the server with a login shell
  to pick up the environment

- :samp:`Restart` ensures the service is restarted on failure

- :samp:`After` makes it depend on network connectivity


.. _how_sdkcraft_mount_interface:

Add plugs and slots
-------------------

.. @artefact interface plug

In |sdk_markup|,
:ref:`interfaces <exp_interface_concepts>` provide a controllable way
of exposing the resources of the host system to the workshops,
and you can use them in a variety of ways
to extend the functionality of your SDK.

For the Ollama SDK, we need several interfaces:
a mount interface to preserve models,
a GPU interface for acceleration,
and a tunnel interface to expose the API server.
The latter is a resource that the SDK itself exposes,
so it will be defined as a slot;
the former two are plugs because they access external resources.

Open :file:`sdk.yaml` again
and add two plugs and a slot to the appropriate sections:

.. code-block:: yaml
   :caption: sdk.yaml
   :emphasize-lines: 12-22

   name: ollama
   version: "0.9.6"
   summary: Get up and running with large language models
   description: |
     Get up and running with Llama 3.3, DeepSeek-R1, Phi-4, 
     Gemma 3, Mistral Small 3.1 and other large language models.
   license: MIT
   platforms:
     ubuntu@22.04:amd64:
     ubuntu@24.04:amd64:

   plugs:
     gpu:
       interface: gpu
     models:
       interface: mount
       workshop-target: /home/workshop/.ollama/models

   slots:
     ollama-server:
       interface: tunnel
       endpoint: 11434


The :samp:`models` plug preserves downloaded models between workshop refreshes.
The :samp:`gpu` plug provides access to GPU acceleration for faster inference,
and the :samp:`ollama-server` slot exposes the Ollama API on port 11434.

.. note::

   You can't explicitly set the *host* directory for mount plugs here;
   this restriction prevents SDKs
   from accessing any arbitrary data on the host file system.
   However, users who add your SDK to their workshops
   will be able to remount the plug elsewhere at run-time.


Add hooks
---------

.. @artefact SDK hook

To prepare the SDK for use,
add the :ref:`hooks <exp_sdk_hooks>`
that run at different stages of the workshop's life cycle,
preparing the SDK for use or preserving its state during updates.

Under :file:`ollama/`,
create a subdirectory
named :file:`hooks/`:

.. code-block:: console

   $ mkdir hooks/
   $ cd hooks/

This directory stores all the hooks for an SDK.


Build: setup base, project
~~~~~~~~~~~~~~~~~~~~~~~~~~

.. @artefact setup-base
.. @artefact setup-project

Under :file:`ollama/hooks/`,
create a file
named :file:`setup-base`:

.. code-block:: shell
   :caption: setup-base

   cat <<EOF >/etc/profile.d/ollama.sh
   export PATH="$SDK/bin:\$PATH"
   EOF


It runs when the workshop is launched or refreshed,
installing system packages and preparing the workshop for use.

.. note::

   For workshops,
   :command:`apt` is configured to exclude recommended or suggested packages
   and answer 'yes' to all confirmation prompts.

   Also, the use of :command:`sudo -u workshop` here is important
   because only the :samp:`setup-project` hook runs as a normal user by default;
   other hooks, like :samp:`setup-base`, run as root.
   Running commands as the non-root user
   helps preserve the correct environment variables and file ownership,
   and can be easier than adjusting permissions afterwards.


In the same directory,
create a file named :file:`setup-project`:

.. code-block:: shell
   :caption: setup-project

   install -D --mode=644 --target-directory ~/.config/systemd/user "$SDK/ollama.service"

   systemctl --user daemon-reload
   systemctl --user enable --now ollama


It runs after :file:`setup-base`,
once the project directory is mounted
and interfaces are connected.
This hook installs and starts the Ollama service as a user service,
ensuring the AI model server is running and ready to use.


Persist: save and restore
~~~~~~~~~~~~~~~~~~~~~~~~~

.. @artefact restore-state
.. @artefact save-state

Some SDKs need to preserve internal state during workshop refresh operations,
such as configuration settings or temporary data that shouldn't be lost.
For these cases,
you would create :file:`save-state` and :file:`restore-state` hooks.

During a :command:`workshop refresh` operation:

- The :file:`save-state` hook runs *before* the workshop is refreshed,
  saving the state of the SDK to :envvar:`$SDK_STATE_DIR`.

- The :file:`restore-state` hook recovers the state
  *after* the workshop has been successfully updated.


However, the Ollama SDK doesn't need these hooks because:

- Downloaded models are stored in the mounted :file:`models/` directory,
  which persists across refreshes

- The :program:`systemd` service configuration
  is stateless and recreated on each refresh

- No custom user configuration needs to be preserved


.. warning::

   The SDK is also refreshed as a part of any workshop refresh operation,
   so any breaking changes in its save-restore logic will cause an error;
   make sure to allow for this in your SDK design.


Report: check health
~~~~~~~~~~~~~~~~~~~~

.. @artefact check-health

Finally, create a hook named :file:`check-health`:

.. @artefact workshopctl

.. code-block:: shell
   :caption: check-health

   output=$(sudo -u workshop --login ollama list 2>&1)
   if [[ $? -eq 0 ]]; then
     workshopctl set-health okay
     exit 0
   fi
   workshopctl set-health error "$output"


It checks whether the Ollama installation is functional
by running :command:`ollama list`.
If it succeeds, the health is set to :samp:`okay`
using the :ref:`workshopctl set-health <exp_workshopctl>` command;
otherwise, it reports the error output from the failed command.

In general, the hook should set the health to :samp:`okay`
and return a zero code if its health checks succeed.
To signal an error, set the health to :samp:`error`
or return a non-zero code.

You can also set the health to :samp:`waiting`
to signal that the hook should be retried for a few seconds.
Unless the hook sets the health to a different value during such a retry,
the health is eventually set to :samp:`error` automatically.


.. _how_sdkcraft_build_sdk:

Package the SDK
---------------

Under :file:`ollama/`, run:

.. code-block:: console

   $ sdkcraft pack


.. @artefact SDK part

This builds all :ref:`SDK parts <exp_sdk_parts>`
defined in the :file:`sdk.yaml` file,
e.g., pulling source code, applying patches, configuring and compiling it
according to the part definition.

.. note::

   For a detailed explanation of the build process,
   see the respective Craft Parts
   `documentation section
   <https://canonical-craft-parts.readthedocs-hosted.com/en/latest/common/craft-parts/explanation/lifecycle/>`_.


Optionally, you can clean the build cache before a build attempt:

.. code-block:: console

   $ sdkcraft clean && sdkcraft pack


.. @artefact SDK metadata

When run without arguments,
:command:`sdkcraft` builds and packs the SDK into files
such as :file:`ollama_amd64_ubuntu@24.04.sdk`,
which contains the build artifacts from the previous step
along with SDK metadata, hooks, and other components.


.. _how_sdkcraft_try:

Try the SDK
-----------

.. @artefact sdkcraft (CLI)
.. @artefact try SDK

When you're confident the SDK builds properly,
you can test it in-place before uploading it to the Store:

.. code-block:: console

   $ sdkcraft try


The command copies the SDK to a special *try area*.
To use it in a workshop, add a prefix: :samp:`try-<NAME>`:

.. code-block:: yaml
   :caption: workshop.yaml

   sdks:
     # ...
     - name: try-ollama


A :samp:`channel` is not needed here;
the SDK is installed from the try area when you refresh the workshop.


.. _how_sdkcraft_publish:

Publish the SDK
---------------

.. @artefact SDK Store

When an SDK is ready, packed, and tried,
the only thing left is to publish it to the SDK Store
for use with |ws_markup|:

.. code-block:: console

   $ sdkcraft.publish ./ollama_amd64_ubuntu@24.04.sdk 24.04/beta


This publishes the newly created SDK
under the :samp:`24.04/beta` channel in the SDK Store.


Use the SDK
-----------

The resulting SDK can be accessed by |ws_markup| as follows:

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 4,5

   name: dev
   base: ubuntu@24.04
   sdks:
     - name: ollama
       channel: 24.04/beta


Note that the workshop :samp:`base`
must match one of the SDK's supported :samp:`platforms`.

.. note::

   Currently, you can't use unpublished SDKs in a workshop.
   However, the :ref:`sketch SDK <tut_sketch_sdks>` and in-project SDKs provide
   a subset of |sdk_markup|'s functionality.


Summary
-------

This was the last step of the entire tutorial series.

You have learned how to create a workshop,
add SDKs to it, and use them in practice.
You have also learned how to sketch a local SDK
and how to craft and publish a full-featured SDK.
You are now familiar with all the basic operations
that |ws_markup| and |sdk_markup| provide
and have had an extensive tour of their capabilities.
