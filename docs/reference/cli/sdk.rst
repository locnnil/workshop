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



.. include:: sdk-find.rst


.. include:: sdk-info.rst


.. include:: sdk-list.rst




.. _ref_sdk__cli_completion:

Shell completion
----------------

The :program:`sdk` CLI ships completion scripts
for Bash, Zsh, Fish, and PowerShell.

.. note::

   When |ws_markup| is installed via snap,
   completion for Bash, Zsh, and Fish is enabled automatically
   for both :program:`workshop` and :program:`sdk`;
   no further configuration is needed for these shells.


To enable completion for the current shell session,
source the script for your shell.

Bash:

.. code-block:: console

   $ source <(sdk completion bash)


Zsh:

.. code-block:: console

   $ source <(sdk completion zsh)


Fish:

.. code-block:: console

   $ sdk completion fish | source


PowerShell:

.. code-block:: console

   $ sdk completion powershell | Out-String | Invoke-Expression


For per-shell installation that persists across new sessions,
follow the instructions printed by the shell-specific help command.
For example, for Bash:

.. code-block:: console

   $ sdk completion bash --help


What gets completed
~~~~~~~~~~~~~~~~~~~

Beyond subcommand and flag names,
the :program:`sdk` CLI completes flag values for :command:`sdk info`:

- :option:`!--base` completes the supported bases
  (for example, :samp:`ubuntu@22.04`, :samp:`ubuntu@24.04`).

- :option:`!--arch` completes the allowed architectures, plus :samp:`all`.


See also
--------

Explanation:

- :ref:`exp_sdk_cli`
