.. meta::
   :description: How-to guide on launching workshops in GitHub Actions
                 using the launch-workshop action on hosted runners,
                 with support for matrix builds and SDK caching.

.. _how_run_workshops_in_github_actions:

How to run workshops in GitHub Actions
======================================

.. @artefact workshop launch
.. @artefact workshop exec

The `launch-workshop <https://github.com/canonical/launch-workshop>`_ action
installs |ws_markup| on a GitHub-hosted runner
and launches an ephemeral workshop for the duration of a job.
Use it to run your project's tests, builds, or other tasks
inside the same workshop you use locally,
without standing up a self-hosted runner.

If you'd rather run jobs on your own hardware,
use a workshop as a self-hosted runner instead;
see :ref:`how_run_github_actions_locally`.


Prerequisites
-------------

Before getting started, ensure you have:

- A GitHub repository with Actions enabled

- A workshop definition committed to the repository,
  at :file:`workshop.yaml` or :file:`.workshop/<NAME>.yaml`

- A `personal access token
  <https://github.com/settings/tokens?type=beta>`_
  with :samp:`Contents: read` permission on :samp:`canonical/workshop`


Configure the workshop token
----------------------------

The action installs |ws_markup| from :samp:`canonical/workshop`,
which is an internal repository.
The token granting read access to that repository
must be stored as an Actions secret in your project repository.

In your project repository on GitHub,
navigate to
:guilabel:`Settings` > :guilabel:`Secrets and variables` > :guilabel:`Actions`,
select :guilabel:`New repository secret`,
and add the token under the name :samp:`WORKSHOP_TOKEN`.

The action reads this secret via the :samp:`token` input.


Add the action to a workflow
----------------------------

The smallest useful workflow
checks out the repository,
launches the default workshop,
and runs a command inside it:

.. code-block:: yaml
   :caption: .github/workflows/test.yaml

   on:
     pull_request:
     push:
       branches: [main]
   jobs:
     test:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v6

         - uses: canonical/launch-workshop@v0
           with:
             token: ${{ secrets.WORKSHOP_TOKEN }}

         - run: workshop exec -- pytest


If the repository contains a single :file:`workshop.yaml`
at the project root,
the action launches it automatically.
For repositories with several workshops under :file:`.workshop/`,
set the :samp:`workshop` input to the one you need.

Pin the action to a major version (:samp:`@v0`)
or a specific commit SHA;
avoid :samp:`@main`,
which can change unexpectedly.


Test across multiple workshops
------------------------------

To test a project against several workshops in parallel,
parameterize the :samp:`workshop` input with a matrix:

.. code-block:: yaml
   :caption: .github/workflows/matrix.yaml
   :emphasize-lines: 6,15

   jobs:
     test:
       runs-on: ubuntu-latest
       strategy:
         matrix:
           workshop: [dev-jammy, dev-noble]
       steps:
         - uses: actions/checkout@v6

         - uses: canonical/launch-workshop@v0
           with:
             token: ${{ secrets.WORKSHOP_TOKEN }}
             workshop: ${{ matrix.workshop }}

         - run: workshop run "$WS" unit-tests
           env:
             WS: ${{ matrix.workshop }}


This pattern fits well for testing the same project
against different Ubuntu releases,
different SDK channels,
or any other axis you encode in the workshop name.


Cache SDK data across runs
--------------------------

Some SDKs expose mount plugs that can be persisted between workflow runs,
such as a package cache or a build cache.
List the available plugs in your local workshop
with :command:`workshop connections`:

.. code-block:: console

   $ workshop connections --all

     Interface  Plug                  Slot              Notes
     mount      dev/python:pip-cache  dev/system:mount  -


Pass the matching :samp:`<SDK>:<PLUG>` lines to the :samp:`cache` input,
one per line:

.. code-block:: yaml
   :caption: .github/workflows/test.yaml
   :emphasize-lines: 5-9

   steps:
     - uses: canonical/launch-workshop@v0
       with:
         token: ${{ secrets.WORKSHOP_TOKEN }}
         cache: |
           cargo:git
           cargo:registry
           go:mod-cache
           python:pip-cache


Each listed plug is mounted from a GitHub-managed cache,
keyed by the SDK and plug name.
Not every SDK defines cacheable plugs;
check the SDK's documentation when in doubt.


Inputs
------

The action exposes the following inputs:

.. list-table::
   :header-rows: 1

   * - Input
     - Description

   * - :samp:`token`
     - Access token for :samp:`canonical/workshop`. Required.

   * - :samp:`version`
     - |ws_markup| version or range of versions. Defaults to :samp:`latest`.

   * - :samp:`project`
     - Directory containing a workshop to launch.
       Defaults to the repository root.

   * - :samp:`workshop`
     - Name of the workshop to launch.
       Required if the project defines several workshops.

   * - :samp:`cache`
     - Mount plugs to cache across runs,
       one :samp:`<SDK>:<PLUG>` entry per line.


Security considerations
-----------------------

When integrating the action into your workflows:

- Store the token as an Actions secret;
  never commit it to the repository or paste it into logs.

- Prefer a fine-grained personal access token
  scoped to :samp:`canonical/workshop`
  with only :samp:`Contents: read` and :samp:`Metadata: read` permissions.

- Pin the action to a major version (:samp:`@v0`) or a commit SHA
  so a compromised tag cannot push unreviewed code into your workflows.

- Rotate :samp:`WORKSHOP_TOKEN` immediately
  if it ever appears in logs, chat history,
  or any other shared transcript.


See also
--------

How-to guides:

- :ref:`how_git_workshops`
- :ref:`how_run_github_actions_locally`
