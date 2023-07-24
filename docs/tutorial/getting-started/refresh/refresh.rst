Updating your workspace
==============================

It is a good idea to keep your locally running workspace instance in sync with
that of your team by using the project's workspace file as a single source of
truth for your development environment. On a change, bring your locally running
workspace instance to the latest revision by running the ``refresh`` command.

The workspace will be rebuilt using the ``base`` and SDKs will be updated from
the respective Store channels:

.. code-block:: bash

    $ workspace refresh nimble
    "nimble" refreshed

If a project contains multiple workspaces, all of them can be refreshed
concurrently. In case of an error, ``refresh`` will automatically abort the
operation and revert all the progress if any of the refresh operations failed.

.. note::

    Any SDK has a notion of state that will be preserved over its life cycle. If
    an SDK had a state data, for example a specific training configuration,
    Workspace will save the state before any refresh operation starts. The state
    will be restored in the refreshed workspace. Both, save and restore scripts
    are provided by the SDK author.
