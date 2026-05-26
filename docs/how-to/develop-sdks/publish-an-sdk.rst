.. _how_publish_sdk:

.. meta::
   :description: How-to guide for publishing a Workshop SDK to the SDK Store with
                 SDKcraft, covering pack, test, try, register, upload, and release,
                 plus how to consume the published SDK from a workshop definition.

How to publish an SDK
=====================

.. @tests in tests/docs-how-to/publish-an-sdk/task.yaml

.. @artefact SDK
.. @artefact SDK Store
.. @artefact sdkcraft (CLI)
.. @artefact try SDK

Publishing turns a packed SDK
into something other |ws_markup| users can pull from the SDK Store.
If the SDK isn't yet packed, tested, and tried locally,
go through :ref:`how_build_sdk` first.

The publishing flow has four steps:

#. **Pack** the SDK into one :file:`.sdk` artifact per platform.
#. **Register** the SDK name on the SDK Store.
#. **Upload** a revision.
#. **Release** the revision to one or more channels.


The first step runs on your machine.
The last three talk to the live SDK Store
at :samp:`api.charmhub.io`
and require an authenticated account.


Prerequisites
-------------

Before starting:

- |sdk_markup| is installed.

- LXD 6.6 or later is running on the host.

- An Ubuntu One account that's allowed to publish SDKs.

- The SDK source tree is clean and ready to build.

- The SDK passes :command:`sdkcraft try` end-to-end
  in at least one workshop.


There is no local-only or dry-run mode for the Store-side commands.
Plan to publish from a workstation with a stable network connection.


Pack the SDK
------------

:command:`sdkcraft pack` builds the SDK and packs it into one artifact
per platform declared in :file:`sdkcraft.yaml`:

.. code-block:: console

   $ sdkcraft pack


The resulting filenames follow the pattern
:file:`<NAME>_<ARCH>_<BASE>.sdk`,
for example :file:`<NAME>_amd64_ubuntu@24.04.sdk`.
:command:`sdkcraft pack` differs from :command:`sdkcraft try`
in one respect:
the artifacts stay in the working directory
rather than being copied into the :ref:`try area <exp_test_try_sdk>`.

If a previous build left state behind,
clean and rebuild from scratch:

.. code-block:: console

   $ sdkcraft clean && sdkcraft pack


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

:command:`sdkcraft init` scaffolds a starter test under
:file:`tests/main/launch/` and a :file:`tests/spread.yaml`
declaring the suites that :command:`sdkcraft test` should pick up.
Add more tests next to the starter,
each in its own subdirectory of the same suite:

.. code-block:: yaml
   :caption: tests/main/smoke/task.yaml

   summary: SDK installs and reports healthy
   execute: |
     workshop launch --verbose --wait-on-error
     workshop info | grep -E 'status:\s+okay'


Try the SDK
-----------

The final pre-publish step is to install the packed SDK
in a real workshop and use it the way an end user would:

.. code-block:: console

   $ sdkcraft try


:command:`sdkcraft try` packs the SDK
and copies it into the :ref:`try area <exp_test_try_sdk>`.
Add it to a workshop with the :samp:`try-` prefix:

.. code-block:: yaml
   :caption: workshop.yaml

   name: dev
   base: ubuntu@24.04
   sdks:
     - name: try-<NAME>


Then launch the workshop and exercise the SDK:

.. code-block:: console

   $ workshop launch --verbose --wait-on-error


This is the last chance to catch problems
before the SDK is on the Store.
Pay particular attention to:

- Hook output in :command:`workshop changes` and :command:`workshop tasks`.

- The SDK's :samp:`status` in :command:`workshop info`;
  a :samp:`waiting` or :samp:`error` state
  is the SDK telling you something is wrong.

- The interaction between this SDK and any other SDKs
  it's meant to be installed alongside.


Register the SDK name
---------------------

Each SDK on the Store has a unique name.
Reserve yours once per SDK, ever.

Authenticate first:

.. code-block:: console

   $ sdkcraft login


Confirm the right account is active:

.. code-block:: console

   $ sdkcraft whoami


Then register the SDK name:

.. code-block:: console

   $ sdkcraft register <NAME>


Names are global to the SDK Store
and normally cannot be re-registered after release.
Pick a name that matches the SDK's :samp:`name` field in :file:`sdkcraft.yaml`
and that you intend to keep.


Upload a revision
-----------------

Each :command:`sdkcraft upload` invocation pushes one :file:`.sdk` file
and assigns it a revision number on the Store:

.. code-block:: console

   $ sdkcraft upload <NAME>_amd64_ubuntu@24.04.sdk


The output reports the revision number.
At this point, the revision is on the Store
but isn't released to any channel yet,
so :command:`sdk find` won't return it.

To upload and release in one step,
pass :option:`!--release` with one or more channels:

.. code-block:: console

   $ sdkcraft upload <NAME>_amd64_ubuntu@24.04.sdk --release latest/edge


Upload one artifact per platform.
If :command:`sdkcraft pack` produced
:file:`<NAME>_amd64_ubuntu@22.04.sdk` and
:file:`<NAME>_amd64_ubuntu@24.04.sdk`,
upload both;
the Store tracks revisions per platform.


Automate uploads from CI
------------------------

The :file:`.github/workflows/` files that ship with the
`canonical/template-sdk <https://github.com/canonical/template-sdk>`_
repository
run :command:`sdkcraft upload --release` automatically
on push to the version branch that :file:`renovate.json` maintains.
After the one-time :command:`sdkcraft register`,
upstream releases land as automated revisions
without further manual commands:
Renovate opens a PR bumping :file:`VERSION`,
the merge of that PR triggers the upload workflow,
and the new revision shows up in the configured channels.

The workflow expects Store credentials
in the repository's GitHub Actions secrets;
configure them once.
For what else the template ships,
see :ref:`how_build_sdk`.


Release a revision
------------------

When a revision is on the Store but not yet released,
or when promoting an existing revision
from one channel to another,
use :command:`sdkcraft release`:

.. code-block:: console

   $ sdkcraft release <NAME> <REVISION> <CHANNELS>


Channels follow the :samp:`[<TRACK>/]<RISK>[/<BRANCH>]` shape:

- :samp:`<TRACK>` is optional and groups related revisions,
  typically along major-version lines or variations in supported platforms
  (for example, :samp:`1.x` or :samp:`nvidia`).
  Omitting it targets the default :samp:`latest` track.

  .. caution::
     Do not use the base (for example, :samp:`24.04`)
     as the track name.
     The SDK Store tracks revisions per platform automatically
     (see the platforms listed in :file:`sdkcraft.yaml`),
     so a per-base track only duplicates that
     and limits how revisions can be grouped meaningfully.

- :samp:`<RISK>` is one of
  :samp:`stable`, :samp:`candidate`, :samp:`beta`, or :samp:`edge`.

- :samp:`<BRANCH>` is optional
  and creates a short-lived channel with a one-month expiration.


Plain :samp:`stable` and comma-separated lists like :samp:`beta,edge`
are valid channel arguments.

For example,
to promote revision 8 to :samp:`latest/stable`:

.. code-block:: console

   $ sdkcraft release <NAME> 8 latest/stable


:command:`sdkcraft release` is idempotent
and never rebuilds or re-uploads;
it only adjusts the channel map.


Consume the published SDK
-------------------------

Once a revision is released to a channel,
any |ws_markup| user can pull it
by referencing the SDK in :file:`workshop.yaml`:

.. code-block:: yaml
   :caption: workshop.yaml

   name: dev
   base: ubuntu@24.04
   sdks:
     - name: <NAME>
       channel: latest/stable


The workshop's :samp:`base`
must match one of the SDK's supported platforms.


See also
--------

Explanation:

- :ref:`exp_sdk_concepts`
- :ref:`exp_test_try_sdk`


How-to guides:

- :ref:`how_build_sdk`


Reference:

- :ref:`ref_sdkcraft_login`
- :ref:`ref_sdkcraft_pack`
- :ref:`ref_sdkcraft_register`
- :ref:`ref_sdkcraft_release`
- :ref:`ref_sdkcraft_test`
- :ref:`ref_sdkcraft_try`
- :ref:`ref_sdkcraft_upload`
- :ref:`ref_workshop_definition`


Tutorial:

- :ref:`tut_craft_sdks`
