Enhancing a workspace
===========================

It is likely that you will make a few iterations on the workspace before arriving at the
required development environment state for your project. ``workspace refresh`` has two modes that
make enhancing and debugging the workspace faster.

With the ``--wait-on-error`` option, the refresh command will not initiate
the operation's abort automatically. Instead, the progress of the operation
will be stopped on the task that caused an error.

.. code-block:: bash

    $ workspace refresh --wait-on-error nimble
    "nimble" refresh failed

It lets you to investigate the issue right at the point it was encoutered. Then,
the refresh can be either continued:

.. code-block:: bash

    $ workspace refresh --continue nimble
    "nimble" refreshed

...or aborted:

.. code-block:: bash

    $ workspace refresh --abort nimble
    "nimble" refreshed
