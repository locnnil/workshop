.. _how_sketch:

How to iterate on SDKs
======================

.. @artefact SDK
.. @artefact SDK definition
.. @artefact workshop run
.. @artefact workshop scripts
.. @artefact workshop sketch-sdk
.. @artefact workshop sketches

Suppose you add an SDK to your workshop but realise it lacks some functionality.
You'd like to extend and maintain it as if it were any other SDK.

Fortunately, |ws_markup| allows you to quickly draft a local SDK
and use it within your workshop. This process is called *sketching*.

Introduction
------------

We'll use the following scenario to demonstrate
how to iterate on an SDK to add missing functionality.

Suppose you're running the :samp:`dev` workshop from the :ref:`tutorial`:

.. code-block:: yaml
   :caption: workshop.yaml

   name: dev
   base: ubuntu@22.04
   sdks:
     - name: go
       channel: jammy/stable


This setup allows you to build and run Go code
while switching between language versions and base images.
However, in real-world usage, your code would be stored in a repository,
and you'd likely use pre-commit checks and linters.
It makes sense to include these in your workshop,
but what if existing SDKs don't provide them?
Should you create and publish an SDK for your personal setup? Probably not.

In this guide, we'll use
`golangci-lint <https://github.com/golangci/golangci-lint>`_
and `shellcheck <https://www.shellcheck.net/>`_ as our tools of choice.
Let's explore how to integrate these utilities into your workshop
in a way that aligns with |ws_markup|.


Start sketching
---------------

Instead of manually installing tools using
:command:`workshop shell` or :command:`workshop exec`,
you can create a local SDK that automates these tasks with |ws_markup|.

Running :command:`workshop sketch-sdk`
opens a simplified version of an :ref:`SDK definition <exp_sdk_definition>`.
This defines all SDK components in a single file named :file:`sdk.yaml`:

.. @artefact workshop sketch-sdk

.. code-block:: console

   $ workshop sketch-sdk


The editor presents a minimal setup with :samp:`name`, :samp:`base`,
and empty :samp:`hooks` and :samp:`plugs`:

.. code-block:: yaml
   :caption: sketch.yaml

   name: sketch
   base: ubuntu@22.04
   # ...


.. note::

   For more details on these components,
   see the :ref:`explanation <exp_index>` section.
   You may want to start with :ref:`exp_sdk` and :ref:`exp_interface`.


To install new software, locate the commented :samp:`setup-base`.
This hook runs when |ws_markup| launches or refreshes the SDK.
Uncomment :samp:`setup-base` and add the installation commands for our tools:

.. code-block:: yaml
   :caption: sketch.yaml

   name: sketch
   base: ubuntu@22.04

   hooks:
     setup-base: |
       apt-get update
       apt-get install shellcheck
       snap install --classic golangci-lint


.. note::

   With |ws_markup|, you don't need to specify non-interactive flags like
   :option:`!-y` or :option:`!--no-install-recommends` with :program:`apt-get`;
   the environment handles this automatically.


Once you save and exit :file:`sdk.yaml`,
|ws_markup| refreshes the workshop, running the new hook:

.. code-block:: console

   Run hook "setup-base" for "sketch" SDK


If errors occur, you can :ref:`debug the installation process
<how_debug_issues_workshops>` as usual with :command:`workshop changes`,
:command:`workshop tasks` and :command:`workshop refresh --continue` or
:command:`workshop refresh --abort`.
Mind that aborting the refresh does not revert your sketched changes,
so you can restart by running :command:`workshop sketch-sdk` again.

After the refresh, the output of :command:`workshop info` should look like this:

.. @artefact sketch SDK

.. code-block:: console

   sketch:
     tracking:   ~/.local/share/workshop/project/b5b0f128/sdk/sketch/dev
     installed:  2025-02-24  (x1)


The sketch SDK entry displays the last update time and its revision (:samp:`x1`).
Instead of a channel, :samp:`tracking` lists the SDK definition path on the host,
distinguishing the sketch SDK from other SDKs.
Each edit with :command:`workshop sketch-sdk` increments the revision number.

At this point, you've created a functional, albeit simple, SDK in minutes.
For more complex needs, you can refine it iteratively.


Add scripts
-----------

To make use of the new functionality in an organised way,
add scripts to run inside your workshop.
These scripts won’t be part of the sketch SDK
but can be executed with :command:`workshop run`.

Edit :file:`workshop.yaml` to include the highlighted lines:

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 7-11

   name: dev
   base: ubuntu@22.04
   sdks:
     - name: go
       channel: jammy/stable
   
   scripts:
     lint: |
       golangci-lint run --out-format=colored-line-number -c .golangci.yaml
     shellcheck: |
       git ls-files | file --mime-type -Nnf- | grep shellscript | cut -f1 -d: | xargs shellcheck


Save and exit. Unlike changes in SDK layout or base,
script updates do not require :command:`workshop refresh`.

Now, instead of typing commands manually and risking typos, you can run:

.. code-block:: console

   $ workshop run lint

     main.go:1:
     ./main.go:5:2: "os" imported and not used (typecheck)
     package main

   $ workshop run shellcheck
   
     In 1.sh line 10:
     cat /etc/passwd | grep root
         ^---------^ SC2002 (style): Useless cat. Consider 'cmd < file | ..' or 'cmd file | ..' instead.


Stash and restore
-----------------

You can temporarily stash the sketch SDK
to revert your workshop to its previous state:

.. code-block:: console

   $ workshop sketch-sdk --stash

.. important::

   Running :command:`workshop sketch-sdk` after stashing overwrites the stash.
   Be cautious to avoid losing your changes.


To restore the stashed SDK:

.. code-block:: console

   $ workshop sketch-sdk --restore

Stashing does not delete the SDK,
allowing you to restore and continue working later.


Craft the SDK
-------------

If you're satisfied with the sketch and think others may benefit from it,
the next possible step is to refine it into a permanent SDK for publishing.
For details, see the |sdk_markup| :ref:`how-to guide <how_sdkcraft>`.


Clean up
--------

To remove the sketch SDK permanently:

.. code-block:: console

   $ workshop sketch-sdk --remove


This deletes all changes introduced by the sketch.

To list all sketch SDKs in a project:

.. code-block:: console

   $ workshop sketches


See Also
--------

Reference:

- :ref:`ref_sdk_definition`
- :ref:`ref_workshop_info`
- :ref:`ref_workshop_scripts`
- :ref:`ref_workshop_sketch-sdk`
- :ref:`ref_workshop_sketches`
