.. _how_build_sdk:

.. meta::
   :description: How-to guide for building a Workshop SDK with SDKcraft, covering
                 scaffolding from the canonical/template-sdk repository, defining
                 parts and interfaces, authoring the hooks, and iterating on
                 the SDK locally.

How to build an SDK
===================

.. @tests in tests/docs-how-to/build-an-sdk/task.yaml

.. @artefact SDK
.. @artefact SDK definition
.. @artefact SDK hook
.. @artefact SDK part
.. @artefact sdkcraft (CLI)
.. @artefact setup-base
.. @artefact setup-project
.. @artefact check-health
.. @artefact save-state
.. @artefact restore-state
.. @artefact try SDK

A standalone SDK is a |sdk_markup| project
with its own :file:`sdkcraft.yaml`,
its own :file:`hooks/` tree,
and the parts and interfaces
that describe what it ships
and how it integrates with workshops.
Build one by laying out the project,
declaring parts and interfaces,
authoring the lifecycle hooks,
and exercising the result locally
before any thought of publishing.


Prerequisites
-------------

Before starting,
ensure the following are in place:

- |sdk_markup| is installed.

- LXD 6.6 or later is running on the host.

- |ws_markup| is installed and configured
  so that you can launch a workshop
  to try the SDK against.


.. _how_build_sdk_scaffold:

Start from the template
-----------------------

New SDKs start from
`canonical/template-sdk <https://github.com/canonical/template-sdk>`_,
a GitHub-template repository
that ships the project skeleton:

- a :file:`sdkcraft.yaml` ready to be filled in,

- a :file:`hooks/` tree with stubs for the lifecycle hooks,

- a :file:`VERSION` file pinning the upstream release the SDK wraps,

- a :file:`renovate.json` that tracks upstream releases on a long-lived
  version branch and opens PRs to bump :file:`VERSION` as they ship,

- CI workflows under :file:`.github/workflows/` that build on pull requests
  and upload to the SDK Store on push to the version branch,

- a README template aligned to the rest of the project shape.


Use it via GitHub's "Use this template" button,
or :command:`git clone` if you don't host on GitHub.
The choice that follows is how to fill the template in.


With the `sdk-designer` skill
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

`template-sdk <https://github.com/canonical/template-sdk>`_
also ships an agentic skill named :samp:`sdk-designer`.
The skill runs an interactive scaffolding conversation:
it asks about the software to package,
the target platforms,
and which interfaces and hooks are needed,
then writes the corresponding files into the template.

#. Aim the agent at the new repository.

#. Run :samp:`/sdk-designer` and answer the prompts.

#. Review the generated files
   and adjust where the skill's defaults don't match your case.


By editing the template directly
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Without the agentic skill,
edit the template files in place.
Fill :file:`sdkcraft.yaml` per :ref:`how_build_sdk_metadata`,
replace each hook stub under :file:`hooks/` per :ref:`how_build_sdk_hooks`,
update :file:`VERSION` to the upstream release you intend to ship first,
and adjust :file:`renovate.json` if the upstream project lives somewhere
other than the GitHub release page the default config targets.


.. _how_build_sdk_metadata:

Fill in the metadata
--------------------

|sdk_markup| needs four pieces of metadata to identify and build the SDK:
:samp:`name`, :samp:`version`, a one-line :samp:`summary`,
and the :samp:`platforms` to build for.
Add :samp:`license` to declare the SDK's licensing terms,
and :samp:`description` for a multi-line write-up:

.. code-block:: yaml
   :caption: sdkcraft.yaml

   name: <NAME>
   version: "<VERSION>"
   summary: One-line description of the SDK
   description: |
     A longer description that explains
     what the SDK packages
     and any noteworthy behavior.
   license: MIT
   platforms:
     ubuntu@22.04:amd64:
     ubuntu@24.04:amd64:


Use the SDK's upstream version for :samp:`version`
when the SDK wraps a single tool;
keep it quoted so that values like :samp:`1.0` aren't parsed as floats.

|sdk_markup| builds one artifact per entry in :samp:`platforms`.
Each entry pairs an Ubuntu base
with a CPU architecture from the Debian naming scheme
(:samp:`amd64`, :samp:`arm64`, and so on).
For SDKs that don't ship compiled binaries,
use :samp:`all` instead of a specific architecture.


Define parts
------------

:ref:`Parts <exp_sdk_parts>` describe how |sdk_markup|
obtains the SDK's payload at build time.
A small SDK often gets by with a single part;
larger SDKs split work along functional boundaries.

For a binary downloaded from a release page,
use the :samp:`dump` plugin with a tarball source:

.. code-block:: yaml
   :caption: sdkcraft.yaml

   parts:
     <NAME>:
       plugin: dump
       source: https://example.com/releases/v${CRAFT_PROJECT_VERSION}/<NAME>-linux-${CRAFT_ARCH_BUILD_FOR}.tar.gz
       source-type: tar


:samp:`${CRAFT_PROJECT_VERSION}` and :samp:`${CRAFT_ARCH_BUILD_FOR}`
expand at build time
from the :samp:`version` field
and the platform |sdk_markup| is currently building for.

If the SDK ships supporting files,
add them as separate parts with the :samp:`file` source type:

.. code-block:: yaml
   :caption: sdkcraft.yaml

   parts:
     <NAME>:
       plugin: dump
       source: https://example.com/releases/v${CRAFT_PROJECT_VERSION}/<NAME>-linux-${CRAFT_ARCH_BUILD_FOR}.tar.gz
       source-type: tar
     service-unit:
       plugin: dump
       source: <NAME>.service
       source-type: file


For source-built SDKs,
the :samp:`rust`, :samp:`go`, and :samp:`python` plugins
take over from :samp:`dump`.
The Craft Parts
`plugin reference <https://documentation.ubuntu.com/craft-parts/latest/reference/plugins/>`_
lists every plugin and its options.


Declare plugs and slots
-----------------------

Plugs and slots wire the SDK
to host resources and to other SDKs in the workshop.
A plug requests access to something the workshop provides;
a slot offers something the SDK exposes.

The most common patterns are:

- A :samp:`mount` plug for cache or model directories
  that should survive :command:`workshop refresh`.

- A :samp:`gpu` plug for SDKs that need GPU acceleration.

- A :samp:`tunnel` slot for services that expose a network endpoint.


For example, an SDK that runs a long-lived HTTP service
and caches data under :file:`~/.cache/<NAME>/`
declares both a plug and a slot:

.. code-block:: yaml
   :caption: sdkcraft.yaml

   plugs:
     cache:
       interface: mount
       workshop-target: /home/workshop/.cache/<NAME>

   slots:
     api:
       interface: tunnel
       endpoint: 8080


The :samp:`workshop-target` value is the in-workshop path
that |ws_markup| backs with persistent host storage;
SDKs can't pick the host path directly,
which prevents them from reaching arbitrary host files.
Workshop users can override the host side at run time
with :command:`workshop remount`.


.. _how_build_sdk_hooks:

Author the hooks
----------------

Hooks are the run-time logic of an SDK.
|ws_markup| runs them at specific lifecycle stages
of the workshop they're installed in.
All hooks are shell scripts in :file:`hooks/<HOOK-NAME>`
and are linted with `ShellCheck <https://www.shellcheck.net/>`_
when the SDK is packed.

:envvar:`SDK` is available inside every hook.
It points to the SDK's installation root inside the workshop;
use it to reference files the SDK ships,
for example :samp:`"$SDK/bin/<NAME>"`.

:envvar:`SDK_STATE_DIR` is available only inside
:samp:`save-state` and :samp:`restore-state`.
It points to a temporary directory
|ws_markup| creates for one :command:`workshop refresh` cycle:
:samp:`save-state` writes to it before the old workshop is destroyed,
and :samp:`restore-state` reads from it
once the new workshop is up.
The directory is gone as soon as the workshop stops,
so don't use it for long-lived data;
back that with a :samp:`mount` plug or store it in the project directory instead.

|ws_markup| recognizes five hook names:
:samp:`setup-base`, :samp:`setup-project`, :samp:`check-health`,
:samp:`save-state`, and :samp:`restore-state`.
A useful SDK rarely needs all five.


setup-base
~~~~~~~~~~

:samp:`setup-base` runs as root
when the SDK is first installed in a workshop
and on every :command:`workshop refresh`.
It is the place for system-wide configuration:
installing :program:`apt` packages,
wiring :envvar:`PATH`,
and laying down service unit files.

A minimal :file:`setup-base` for a single-binary SDK
adds the SDK's :file:`bin/` directory to the system :envvar:`PATH`:

.. code-block:: shell
   :caption: hooks/setup-base

   cat <<EOF > /etc/profile.d/<NAME>.sh
   export PATH="$SDK/bin:\$PATH"
   EOF


Place system-wide environment variables under :file:`/etc/profile.d/`
so they apply across shells.
Avoid editing :file:`/etc/bash.bashrc` directly;
|ws_markup| may support more than one shell
and :file:`/etc/profile.d/` is the portable seam.

Inside :samp:`setup-base`,
:program:`apt` is preconfigured to skip recommended and suggested packages
and to answer "yes" to confirmation prompts,
so :command:`apt-get install` calls can be terse:

.. code-block:: shell
   :caption: hooks/setup-base

   apt-get update
   apt-get install build-essential cmake ninja-build


Operations performed in :samp:`setup-base` become part of the workshop's
:ref:`base snapshot <exp_workshop_definition_sdks>`,
so subsequent refreshes start from a warmed-up state.


setup-project
~~~~~~~~~~~~~

:samp:`setup-project` runs as the :samp:`workshop` user
after :samp:`setup-base`,
once the project directory is mounted
and interface plugs and slots are connected.
This is the right place for per-user configuration:
activating virtual environments,
enabling user :program:`systemd` services,
and writing files under :file:`/home/workshop/`.

A typical :file:`setup-project` for an SDK that ships a service unit
installs and starts the service as a user-level systemd unit:

.. code-block:: shell
   :caption: hooks/setup-project

   install -D --mode=644 --target-directory ~/.config/systemd/user \
       "$SDK/<NAME>.service"

   systemctl --user daemon-reload
   systemctl --user enable --now <NAME>


User-level :program:`systemd` services are preferred over root-level ones
because they cleanly tie their lifetime
to the :samp:`workshop` user's session
and don't require :samp:`sudo`.

Operations in :samp:`setup-project` don't go into the base snapshot,
so use it for anything that depends on project-specific state
or that should be re-evaluated on every launch.


check-health
~~~~~~~~~~~~

:samp:`check-health` runs as root once every other hook has finished:
on :command:`workshop launch`,
after :samp:`setup-project` has run for every SDK in the workshop;
on :command:`workshop refresh`,
after :samp:`restore-state` has run for every SDK.
|ws_markup| also re-runs :samp:`check-health` on demand
when it reassesses the workshop's state.
Use it to verify the SDK is functional
and to report status back through :command:`workshopctl set-health`.

The canonical pattern is to exercise a real entry point
and channel any error output back to the user:

.. code-block:: shell
   :caption: hooks/check-health

   if ! output=$(sudo -u workshop --login <NAME> --version 2>&1); then
     workshopctl set-health error "$output"
     exit
   fi
   workshopctl set-health okay


Run the command as :samp:`sudo -u workshop --login`
so it picks up the same environment
that a workshop user would see interactively;
this catches PATH wiring bugs in :samp:`setup-base`
that would otherwise stay hidden.

Three health states are meaningful:

- :samp:`okay`: The SDK is functional.

- :samp:`error`: Something is wrong.
  Supply a message that helps a user understand what failed.

- :samp:`waiting`: The hook should be retried.
  |ws_markup| retries up to ten times, once per second.
  If the SDK never reaches :samp:`okay` or :samp:`error`,
  the health flips to :samp:`error` after those retries are exhausted.


save-state and restore-state
~~~~~~~~~~~~~~~~~~~~~~~~~~~~

:samp:`save-state` and :samp:`restore-state` are an optional pair
that only runs at :command:`workshop refresh`.
:samp:`save-state` runs in the old SDK revision,
before |ws_markup| destroys the old workshop.
:samp:`restore-state` runs in the new SDK revision,
after :samp:`setup-project` has finished for every SDK.
Their job is to carry data
across the refresh boundary in :envvar:`SDK_STATE_DIR`.

Because :samp:`restore-state` runs after :samp:`setup-project`,
restored files aren't yet present
while :samp:`setup-project` is still executing;
keep any setup that depends on restored state
inside :samp:`restore-state` itself,
or have :samp:`check-health` retry by reporting :samp:`waiting`
until the state shows up.

Use them when the SDK keeps configuration or transient data
that doesn't already live in a mount plug or a project file.
Both hooks run as :samp:`root`,
so reference the :samp:`workshop` user's home explicitly
rather than relying on :samp:`~`:

.. code-block:: shell
   :caption: hooks/save-state

   if [ -d /home/workshop/.config/<NAME> ]; then
     cp -a /home/workshop/.config/<NAME> "$SDK_STATE_DIR/config"
   fi


.. code-block:: shell
   :caption: hooks/restore-state

   if [ -d "$SDK_STATE_DIR/config" ]; then
     install -d -o workshop -g workshop /home/workshop/.config/<NAME>
     cp -fa "$SDK_STATE_DIR/config/." /home/workshop/.config/<NAME>/
     chown -R workshop:workshop /home/workshop/.config/<NAME>
   fi


Skip these hooks entirely when:

- The SDK has no state worth preserving,
  for example a stateless CLI tool.

- The state already lives in a directory
  backed by a :samp:`mount` plug,
  which survives refreshes by definition.

- The state is regenerated cheaply by :samp:`setup-base`
  or :samp:`setup-project`.


.. warning::

   The SDK itself is refreshed as part of any :command:`workshop refresh`.
   A bug in :samp:`save-state` or :samp:`restore-state`
   becomes a workshop-wide refresh failure,
   so test these hooks aggressively
   before relying on them.


.. _how_build_sdk_try:

Try the SDK
-----------

Once the definition and hooks are in place,
build and install the SDK into a workshop with :command:`sdkcraft try`:

.. code-block:: console

   $ sdkcraft try


|sdk_markup| packs the SDK for each declared platform
into files of the form :file:`<NAME>_<ARCH>_<BASE>.sdk`
and copies them into the :ref:`try area <exp_test_try_sdk>`.

Add the SDK to a workshop definition
using the :samp:`try-` prefix:

.. code-block:: yaml
   :caption: workshop.yaml

   name: dev
   base: ubuntu@24.04
   sdks:
     - name: try-<NAME>


The :samp:`base` must match one of the SDK's :samp:`platforms`.
Then launch the workshop with verbose output
and a wait-on-error breakpoint
so that any hook failure leaves a usable container behind for inspection:

.. code-block:: console

   $ workshop launch --verbose --wait-on-error


Pay particular attention to:

- Hook output in :command:`workshop changes` and :command:`workshop tasks`.

- The SDK's :samp:`status` in :command:`workshop info`;
  a :samp:`waiting` or :samp:`error` state
  is the SDK telling you something is wrong.

- The interaction between this SDK and any other SDKs
  it's meant to be installed alongside.


On success,
:command:`workshop info` reports the SDK
and a :samp:`status` of :samp:`okay`.
On failure,
:command:`workshop changes` and :command:`workshop tasks`
point at the hook that failed;
see :ref:`how_debug_issues_workshops` for the full troubleshooting flow.


Test the SDK
------------

If the SDK ships a :file:`tests/` directory with
`spread <https://github.com/canonical/spread>`__ tests,
run them against the freshly packed artifacts:

.. code-block:: console

   $ sdkcraft test


|sdk_markup| provisions a clean LXD container for each test,
installs the packed SDK into a workshop,
and runs the declared scenarios end-to-end.

Tests live under :file:`tests/`,
organised in suites declared by :file:`tests/spread.yaml`.
The starter test at :file:`tests/main/launch/` illustrates the layout;
add more tests next to the starter,
each in its own subdirectory of the same suite:

.. code-block:: yaml
   :caption: tests/main/smoke/task.yaml

   summary: SDK installs and reports healthy
   execute: |
     workshop launch --verbose --wait-on-error
     workshop info | grep -E 'status:\s+okay'


Iterate
-------

Normally, you would use the :command:`workshop sketch-sdk` command
to iterate on an SDK locally.
However, even when it doesn't fit your purpose,
the build-try-fix loop is fast:

#. Edit the definition or a hook.

#. Run :command:`sdkcraft clean && sdkcraft try`
   to rebuild from a clean state.

#. Run :command:`workshop refresh`
   to reapply the SDK in the existing workshop,
   or :command:`workshop launch --verbose --wait-on-error`
   for a fresh start.


:command:`sdkcraft clean` is optional;
omit it when the change is small enough
that |sdk_markup| can incrementally rebuild.
For build internals, see the Craft Parts
`lifecycle documentation
<https://documentation.ubuntu.com/craft-parts/latest/common/craft-parts/explanation/lifecycle/>`_.


Next steps
----------

When the SDK behaves correctly under :command:`sdkcraft try`
and its test suite passes,
proceed to :ref:`how_publish_sdk`
to register the SDK name on the SDK Store and upload a revision.


See also
--------

Explanation:

- :ref:`exp_best_interfaces`
- :ref:`exp_best_parts_decomposition`
- :ref:`exp_best_parts_or_hooks`
- :ref:`exp_interfaces`
- :ref:`exp_sdk_best_practices`
- :ref:`exp_sdk_concepts`
- :ref:`exp_sdk_hooks`
- :ref:`exp_sdk_parts`
- :ref:`exp_test_try_sdk`
- :ref:`exp_workshopctl`
- :ref:`exp_workshop_definition_sdks`


Reference:

- :ref:`ref_sdk_definition`
- :ref:`ref_sdk_hooks`
- :ref:`ref_sdk_parts`
- :ref:`ref_workshop_sketch-sdk`
- :ref:`ref_workshopctl__cli`


Tutorial:

- :ref:`tut_craft_sdks`
