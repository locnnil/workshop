.. _exp_ssh_agent_interface:

SSH agent interface
===================

The SSH agent interface
enables access to the host system's SSH agent
from inside the workshop
to let it use the host's SSH keys and configuration securely.


SSH interface plug
------------------

An essential element here is the SSH interface plug
that is declared in the SDK definition.
By adding it,
the SDK publisher lets the workshop connect to the host's SSH agent,
which may come in handy in various SDK-specific tasks
such as cloning private repositories, accessing remote machines and so on.


SSH interface slot
------------------

To enable this mechanism,
|project_markup| provides an SSH interface slot
that multiple SSH interface plugs
can :ref:`connect <exp_interface_connections>` to.


When an SDK is installed
during :command:`launch` and :command:`refresh`,
|project_markup| checks that the plug that targets the slot
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


This means a proxy Unix domain socket is created inside the workshop
and a corresponding :envvar:`SSH_AUTH_SOCK` value
is set for the workshop's users:

.. code-block:: console

   $ workshop shell ws
   workshop@ws-8584e571$ echo $SSH_AUTH_SOCK

     /var/lib/workshop/ws-ssh-agent.ssh


So the host's SSH identities and configurations
become available inside the workshop:

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
