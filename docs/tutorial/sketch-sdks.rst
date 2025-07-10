.. _tut_sketch_sdks:

.. meta::
   :description: Tutorial on creating experimental SDKs with the 'workshop sketch-sdk'
                 command, enabling quick local SDK experiments without publishing them.

Customize with sketch SDKs
==========================

This is the second section of the :ref:`three-part series <tut_index>`;
it teaches you to create experimental SDKs quickly
using the :command:`workshop sketch-sdk` command
to run local SDK experiments without publishing them.
It relies on the knowledge gained in the :ref:`tut_get_started` section,
where you learned how to create and run workshops.

Suppose you built your workshop with a number of SDKs
only to realize it still lacks some functionality you need.
Naturally, you'd like to add that,
but can you align it with the way |ws_markup| operates?

.. @artefact SDK

Fortunately, |ws_markup| allows you to quickly draft a local SDK
and use it within your workshop. This process is called *sketching*.

.. note::

   For details of how sketch SDKs are different from regular SDKs,
   see the :ref:`exp_sketch_sdk` explanation section.


Introduction
------------

We'll use the following scenario to demonstrate
how to iterate on an SDK to add missing functionality.

Suppose you're running the :samp:`dev` workshop
from the :ref:`tut_get_started` tutorial section:

.. code-block:: yaml
   :caption: workshop.yaml

   name: dev
   base: ubuntu@22.04
   sdks:
     - name: go
       channel: jammy/stable


This setup allows you to build and run Go code
while switching between language versions and base images.
However, in real-world usage, your project code would be stored in a repository,
and you'd likely use pre-commit checks and linters.
But what if existing SDKs in your workshop don't provide these checks?
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

.. @artefact SDK definition

Running :command:`workshop sketch-sdk`
opens a simplified version of an :ref:`SDK definition <exp_sdk_definition>`.
This defines all SDK components in a single file named :file:`sdk.yaml`:

.. @artefact workshop sketch-sdk

.. code-block:: console

   $ workshop sketch-sdk


.. note::

   The :command:`workshop sketch-sdk` command opens the SDK definition
   in your default text editor.
   To use a specific editor,
   set the :envvar:`EDITOR` environment variable, e.g.:

   .. code-block:: console

      $ export EDITOR=vim
      $ workshop sketch-sdk


The editor presents a minimal setup
with empty :samp:`hooks`, :samp:`plugs`, and :samp:`slots`:

.. code-block:: yaml
   :caption: sdk.yaml

   name: sketch

   hooks:
    # ...
   plugs:
    # ...
   slots:
    # ...


.. note::

   For more details on these components,
   see the :ref:`explanation <exp_index>` section.
   You may want to start with :ref:`exp_sdks` and :ref:`exp_interface`.


To install new software, locate the commented :samp:`setup-base`.
This hook runs when |ws_markup| launches or refreshes the SDK.
Uncomment :samp:`setup-base` and add the installation commands for our tools:

.. code-block:: yaml
   :caption: sdk.yaml

   name: sketch

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
:command:`workshop tasks`, and :command:`workshop refresh --continue` or
:command:`workshop refresh --abort`.
Mind that aborting the refresh does not revert your sketched changes,
so you can restart by running :command:`workshop sketch-sdk` again.

After the refresh, the output of :command:`workshop info` should look like this:

.. @artefact sketch SDK

.. code-block:: console

   sketch:
     tracking:   ~/.local/share/workshop/id/b5b0f128/dev/sdk/sketch/current
     installed:  2025-02-24  (x1)


The sketch SDK entry displays the last update time and its revision (:samp:`x1`).
The SDK is local, so :samp:`tracking` lists the SDK definition path on the host;
each edit with :command:`workshop sketch-sdk` increments the revision number.

At this point, you've created a functional, albeit simple, SDK in minutes.
For more complex needs, you can refine it iteratively.

.. note::

   You can only have one sketch SDK per workshop at a time;
   there's no way to create :samp:`sketch-foo`, :samp:`sketch-draft`,
   :samp:`sketch-final-final`, and so on.


Stash and restore
-----------------

You can temporarily stash the sketch SDK
to revert your workshop to its pre-sketching state:

.. code-block:: console

   $ workshop sketch-sdk --stash

.. important::

   Stashing does not delete the SDK,
   allowing you to restore it and continue working later.

   However, there's only one slot available for stashing.
   Running :command:`workshop sketch-sdk --stash` overwrites the existing stash,
   if any.
   Be cautious to avoid losing your changes.


To restore the stashed SDK:

.. code-block:: console

   $ workshop sketch-sdk --restore


Eject the SDK
-------------

.. @artefact in-project SDK
.. @artefact SDK Store

If you're satisfied with the sketch to a degree
where others may benefit from it,
you can either add it to your project
or publish it in the SDK Store.

The first step is to *eject* the SDK:

.. code-block:: console

   $ workshop sketch-sdk --eject

     "dev" sketch ejected to ".workshop/hello-workshop"
     To use it, add "project-hello-workshop" to the list of SDKs and run 'workshop refresh dev'


This moves the SDK definition files
into the :file:`.workshop/` subdirectory of the project
and removes the sketch from the running workshop.
|ws_markup| can pull SDKs from this directory,
bypassing the SDK Store.
Such SDKs are called *in-project SDKs*.

.. note::

   For details of how in-project SDKs are different from regular and sketch SDKs,
   see the :ref:`exp_in_project_sdk` explanation section.

   If you'd rather publish a regular SDK for other projects to use,
   |sdk_markup| can help;
   proceed to the next part of the tutorial,
   :ref:`tut_craft_sdks`.


The new SDK is named after the project directory by default;
use the :option:`!--name` option to change this:

.. code-block:: console

   $ workshop sketch-sdk --eject --name tools

     "dev" sketch ejected to ".workshop/tools"
     To use it, add "project-tools" to the list of SDKs and run 'workshop refresh dev'


After you eject the SDK,
add it to your workshop definition
(usually in :file:`workshop.yaml`) under the :samp:`sdks:` list:

.. code-block:: yaml

   sdks:
     - name: project-tools


This is required for the workshop to use the new in-project SDK;
next, run :command:`workshop refresh` to apply the change.

The definition and the hooks of the ejected SDK
are placed in the :file:`.workshop/tools/` subdirectory of the project.
If your project did not previously have a :file:`.workshop/` directory
(for example, if you only had a single :file:`workshop.yaml` in the project root),
add the new directory and its contents to version control manually:

.. code-block:: console

   $ git add .workshop/
   $ git commit -m "Add tools project SDK"


This ensures your in-project SDK is tracked
and can be shared with collaborators or CI systems.


Clean up
--------

To remove the sketch SDK permanently:

.. code-block:: console

   $ workshop sketch-sdk --remove


This deletes all changes introduced by the sketch.
Also, mind that :command:`workshop remove` removes the sketch SDK,
as you could expect,
including the stashed version.

To list all sketch SDKs in a project:

.. @artefact workshop sketches

.. code-block:: console

   $ workshop sketches


A project can have multiple workshops, remember;
hence the need to browse the respective sketches.


Next steps
----------

This was the last step in this tutorial section;
you are now familiar with the essentials of sketching in |ws_markup|
and have had your first taste of what it can achieve.

If you've mastered sketching local SDKs,
your next logical step is to explore
how to create publicly available SDKs:

- :ref:`tut_craft_sdks`:
  Go through the complete process
  of building and publishing full-fledged SDKs to the SDK Store.
  This tutorial section covers the workflow for creating production-ready SDKs
  that can be shared with others.


This section builds on what you've learned here
and expands your |ws_markup| skills.
