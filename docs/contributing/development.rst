:relatedlinks: [Spread](https://github.com/canonical/spread), [golangci-lint](https://golangci-lint.run/), [gocheck](https://pkg.go.dev/gopkg.in/check.v1#section-readme)

.. _contributing_development:

.. meta::
   :description: Make a code change to Workshop. Set up, develop, test, and
                 review your change within the project's standard workflow.

Contribute to development
=========================

A code change to |ws_markup| moves from a local development environment,
through testing and review, to a merged pull request.


Set up your work environment
----------------------------

|ws_markup| has a client-server architecture.
Its :program:`workshopd` daemon exposes a RESTful API
(see :file:`internal/daemon/api.go`)
to the clients.

The recommended way to run the current sources
is the :command:`go tool try` development tool
wired into :file:`go.mod`:

.. code-block:: console

   $ go tool try

This builds :file:`./cmd/...`
into a temporary session directory under :file:`try_sessions/`,
starts :program:`workshopd` against it,
and drops you into a subshell
with :envvar:`WORKSHOP`, :envvar:`WORKSHOP_CACHE`,
:envvar:`WORKSHOP_SOCKET`, and :envvar:`PATH` pre-configured.
Exit the shell to tear the session down.
Pass :option:`!--keep` to retain the session directory for inspection.
Run :command:`go tool try` again from inside the shell
to rebuild and restart :program:`workshopd` in place.

To run :program:`workshopd` directly:

.. code-block:: console

   $ go install ./cmd/...
   $ export WORKSHOP=~/workshop
   $ export WORKSHOP_CACHE=~/workshop-cache
   $ export WORKSHOP_DEBUG=1
   $ workshopd run --create-dirs

The client can connect using the daemon's Unix domain socket:

.. code-block:: console

   $ export WORKSHOP=~/workshop
   $ workshop list

`Spread <https://github.com/canonical/spread>`__ is the end-to-end testing tool
for |ws_markup|.
Install it from GitHub:

.. code-block:: console

   $ git clone https://github.com/canonical/spread
   $ cd spread
   $ go install ./...

Make sure the :file:`$GOPATH/bin/` directory
is included in :envvar:`PATH`.
After successful installation, you should see the help message by running:

.. code-block:: console

   $ spread -h

Run the documentation end-to-end test suites with:

.. code-block:: console

   $ spread tests/docs-tutorial/
   $ spread tests/docs-how-to/

The tutorial suite requires an |sdk_markup| snap
in the :file:`tests/` directory
with a filename matching :file:`sdkcraft_*.snap`.
Download the latest |sdk_markup| release from the
`repository <https://github.com/canonical/sdkcraft/releases>`_
and move it there before running the suite.


Choose a task
-------------

Work is tracked in the project's
`issue tracker <https://github.com/canonical/workshop/issues>`__.
Browse the open issues and comment on one to signal that you're taking it on.

For a small, self-contained fix, you can open a pull request directly.
For a larger or more involved change, open an issue first
to agree on the approach with the maintainers
before you invest time in the implementation.


Draft your work
---------------

Create a branch
~~~~~~~~~~~~~~~

Name your branch after the type of change and a short description.
|ws_markup| uses the commit type as a branch prefix, for example:

- :samp:`feat/workspace-start`
- :samp:`fix/spread-tests-github`
- :samp:`chore/update-lxd`


Develop
~~~~~~~

Keep each change focused and reversible.
When a decision might be costly to reverse,
state the rationale in the pull request description
so reviewers can follow the reasoning and collaborate on it.

|ws_markup| follows a set of Go conventions for naming, error handling,
error messages, and code structure.
See the :ref:`coding_style_guide` for the full set of patterns and their rationale.


Test
~~~~

Run the unit and integration tests before submitting a pull request.
|ws_markup| tests use
`gocheck <https://pkg.go.dev/gopkg.in/check.v1#section-readme>`_,
which integrates with :command:`go test`:

.. code-block:: console

   $ go test ./...
   $ go test -check.f <TEST-NAME|SUITE-NAME>

Run the end-to-end and integration tests with
`Spread <https://github.com/canonical/spread>`__:

.. code-block:: console

   $ spread tests/<TEST-PATH-NAME>

To check code coverage:

.. code-block:: console

   $ go test -coverpkg=<./...|PACKAGE> \
       -covermode=<set|count|atomic> \
       -coverprofile=<OUTPUT-FILE> <./...|PACKAGE>

For example, to measure coverage using all tests:

.. code-block:: console

   $ go test -covermode=count -coverpkg=./... -coverprofile=cover.out ./...

To generate an HTML representation:

.. code-block:: console

   $ go tool cover -html=<OUTPUT-FILE> -o <OUTPUT-HTML>

For example:

.. code-block:: console

   $ go tool cover -html=cover.out -o cover.html

The output flag can be omitted to open in the default browser:

.. code-block:: console

   $ go tool cover -html=cover.out

The commands above work for unit and integration tests
instrumented directly with :command:`go test`.
Integration tests run through :program:`spread`
create the coverage profile automatically,
but the artifacts need to be collected from the VM
with the :option:`!-artifacts` flag:

.. code-block:: console

   $ spread -artifacts=<PATH-TO-DEST> tests/integration/

Formatting and common pitfalls are checked with
`golangci-lint <https://golangci-lint.run/>`_. Run the checks locally:

.. code-block:: console

   $ golangci-lint run

Some issues can be fixed automatically:

.. code-block:: console

   $ golangci-lint run --fix

If `pre-commit <https://pre-commit.com/index.html#install>`_ is available,
:program:`git` can run these checks on every commit:

.. code-block:: console

   $ pre-commit install


Document your work
~~~~~~~~~~~~~~~~~~

Update the documentation to match your change:
new behavior, changed flags, or removed features all belong in the docs.
See :ref:`contributing_documentation` for how to write, build, and test them.


Commit
~~~~~~

|ws_markup| commit messages differ
from conventional commits in capitalization:

.. code-block:: none

   Ensure correct permissions and ownership for the content mounts

    * Work around an LXD issue regarding empty dirs:
      https://github.com/canonical/lxd/issues/12648

    * Ensure the source directory is owned by the user running a workshop.

   Links:
   - ...
   - ...

The messages rarely, if ever, state the type of the commit
(:samp:`fix`, :samp:`feat`, and so on);
these are used for branch naming instead.


Review with the team
--------------------

Send for review
~~~~~~~~~~~~~~~

Open a `pull request <https://github.com/canonical/workshop/pulls>`_
against the repository.
Fill in the pull request template checklist,
which largely reiterates the expectations covered here.


Address quality concerns
~~~~~~~~~~~~~~~~~~~~~~~~

Your pull request runs a set of automatic checks for linting, unit tests,
end-to-end tests, and security scanning.
For the full catalog of workflows, see :ref:`contributing_cicd`.

After receiving review comments, optimize for commit history clarity.
Address them with
`fixup commits <https://git-scm.com/docs/git-commit/2.32.0#Documentation/git-commit.txt---fixupamendrewordltcommitgt>`_
and rebase using
`autosquash <https://git-scm.com/docs/git-rebase#Documentation/git-rebase.txt---autosquash>`_
when reasonable.


Wrap up the review
~~~~~~~~~~~~~~~~~~

Once the checks pass and a maintainer approves the change,
it's merged into the target branch.
Keep an eye on the pull request in case follow-up questions come up.


Get help and support
--------------------

If you get stuck, ask on the issue you're working on,
or open a new one in the
`issue tracker <https://github.com/canonical/workshop/issues>`__.
