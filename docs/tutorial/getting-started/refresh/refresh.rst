Updating a workspace
===========================

The SDK content that constitutes a workspace can be updated from their channels
by using a ``refresh`` command. The refresh command will only succeed if all the
tasks that constitute the operation are successful. In the case of an error,
``refresh`` will automatically abort the operation and revert all the progress.

.. code-block:: bash

    $ workspace refresh nimble
    "nimble" refreshed
