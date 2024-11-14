:slug: ref-workshop-cli

.. _ref_cli:

CLI commands
============

The :program:`workshop` utility
exposes the following commands,
each with its own set of options,
and also has a number of global options
such as :option:`!--help` or :option:`!-h`:

.. toctree::
   :glob:
   :maxdepth: 1

   workshop-*


Shell completion
----------------

To configure shell completion,
follow the instructions offered by :command:`workshop completion`:

.. code-block:: console

   $ workshop completion -h


For example, in your :file:`~/.bashrc` file:

.. code-block:: shell

   source <(workshop completion bash)


See also
--------

Explanation:

- :ref:`exp_cli`
