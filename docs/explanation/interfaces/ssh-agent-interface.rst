.. _exp_ssh_agent_interface:

SSH agent interface
===================

The SSH agent interface
provides access to the host system's SSH agent
from inside the workshop,
allowing it to securely use the host's SSH keys and configuration.


SSH interface plug
------------------

An essential element here is the content interface plug,
which is declared in the :ref:`SDK definition <exp_sdk_definition>`
and is thus beyond the reach of |project_markup|.
By adding it,
the SDK publisher allows the workshop to connect to the host's SSH agent,
which can be useful in various SDK-specific tasks
such as cloning private repositories, accessing remote machines and so on.


SSH interface slot
------------------

To enable this mechanism,
|project_markup| provides an SSH interface slot
to which multiple SSH interface plugs can
:ref:`connect <exp_interface_connections>`.

When an SDK is installed
during :command:`launch` and :command:`refresh`,
|project_markup| checks that the plug targeting the slot
passes :ref:`validation <exp_interfaces_validation>`;
if it does,
it can be manually connected with the :command:`connect` command:

.. code-block:: console

   $ workshop connect ws/ssh-sdk:ssh-agent


To make sure the plug has connected to the slot:

.. code-block:: console

   $ workshop connections --all

     Interface  Plug                   Slot        Notes
     ...
     ssh-agent  ws/ssh-sdk:ssh-agent   :ssh-agent  manual


This means a proxy Unix domain socket has been created inside the workshop
and a corresponding :envvar:`SSH_AUTH_SOCK` value
has been set for the workshop's users:

.. code-block:: console

   $ workshop shell ws
   workshop@ws-8584e571$ echo $SSH_AUTH_SOCK

     /var/lib/workshop/ws-ssh-agent.ssh


So the host's SSH identities and configuration
are available inside the workshop:

.. code-block:: console

   workshop@ws-8584e571$ ssh-add -l

     4096 SHA256:cb19/bE/6irqhII1KbQqRmo1royWi58qcUD9MEn/9fE user@example.com (RSA)


See also
--------

Explanation:

- :ref:`exp_interfaces`
- :ref:`exp_plugs_slots`
- :ref:`exp_sdk_definition`
- :ref:`exp_workshop_def`


Reference:

- :ref:`ref_workshop_connect`
- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_shell`
