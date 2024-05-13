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

.. important::

   An SDK definition, usually named :file:`sdkcraft.yaml`,
   is different from a
   :ref:`workshop definition <exp_workshop_def>`,
   usually named :file:`.workshop.<NAME>.yaml`;
   the former is used to build SDKs with `SDKcraft`_
   and isn't normally needed with |project_markup|,
   whereas the latter is a crucial element of daily |project_markup| activities.

   The following example is provided only to detail how the SSH interface works.


A basic structure includes the name of the plug itself
and the name of the interface (:samp:`ssh-agent` in this case):

.. code-block:: yaml
   :caption: sdkcraft.yaml

   name: ssh-sdk
   title: SSH SDK
   base: ubuntu@22.04
   summary: The SSH agent SDK
   description: |
     SSH agent SDK serves to demonstrate how the SSH agent interface works.

   plugs:
     ssh:
       interface: ssh-agent


This definition creates a plug called :samp:`ssh`
that sets its :samp:`interface` type to :samp:`ssh-agent`,
which makes it an SSH interface plug.


SSH interface slot
------------------

To let the workshop access the host system's SSH agent,
|project_markup| creates an SSH interface slot,
which multiple SSH agent interface plugs can then access.

When an SDK is installed
during :command:`workshop launch` and :command:`workshop refresh`,
|project_markup| checks that the plug that targets the slot can be installed;
if yes, it can be manually connected with :command:`workshop connect`:

.. code-block:: console

   $ workshop connect ws/ssh-sdk:ssh-agent


If the plug passes the checks, it's successfully connected to the slot:

.. code-block:: console

   $ workshop connections --all

       Interface  Plug                   Slot        Notes
       ...
       ssh-agent  ws/ssh-sdk:ssh-agent   :ssh-agent  manual


Then, a proxy Unix domain socket is created inside the workshop
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

- :ref:`exp_sdk_definition`
- :ref:`exp_interfaces_plugs_slots`
- :ref:`exp_workshop_def`


Reference:

- :ref:`ref_workshop_connect`
- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_disconnect`
- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_refresh`
