.. _ref_sdk__cli:

.. meta::
   :description: Overview of the "sdk" CLI utility, listing available
                 commands and global options for managing SDKs.

sdk (CLI)
=========

.. @artefact sdk (CLI)

The :program:`sdk` utility exposes the following commands,
each with its own set of options,
and also has a number of global flags:

-h, --help

   Print the help message for the command.


-v, --version

   Print SDK CLI version.


{{ range .Files }}
.. include:: {{ . }}

{{ end }}


.. _ref_sdk__cli_completion:

Shell completion
----------------

To configure shell completion,
follow the instructions offered by :command:`sdk completion`:

.. code-block:: console

   $ sdk completion -h

For example, in your :file:`~/.bashrc` file:

.. code-block:: console

   $ source <(sdk completion bash)


See also
--------

Explanation:

- :ref:`exp_sdk_cli`
