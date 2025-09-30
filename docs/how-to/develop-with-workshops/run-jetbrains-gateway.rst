.. _how_jetbrains_gateway:

.. meta::
    :description: How-to guide on running JetBrains Gateway with Workshop
                  for remote development.

How to use JetBrains Gateway with Workshop
==========================================

`JetBrains Gateway
<https://www.jetbrains.com/remote-development/gateway/>`_
allows you to connect to remote development environments
while using your favorite JetBrains IDEs (IntelliJ IDEA, PyCharm, and so on).
This guide explains how to use JetBrains Gateway with |ws_markup|
for remote development by connecting to your workshop.


Prerequisites
-------------

- Download and install `JetBrains Gateway
  <https://www.jetbrains.com/remote-development/gateway/>`__
  on your host system.

- Create a workshop or choose an existing one.

- Generate an SSH key pair if you don't have one already:

  .. code-block:: console

     $ ssh-keygen -t rsa -b 4096 -C "<your_email@example.com>"


Configure your workshop
-----------------------

Configure your workshop to accept SSH connections
by adding a plug, a slot, and an action to upload your public SSH key.

#. First, add a tunnel interface plug for the system SDK
   in the workshop definition:

   .. code-block:: yaml
      :caption: workshop.yaml

      sdks:
        - name: system
          plugs:
            gateway:
              interface: tunnel
              endpoint: 2200


   This exposes port :samp:`2200` on the host
   that you will use in JetBrains Gateway.


#. Next, add a corresponding slot;
   you can graft it onto an existing SDK
   or add it with sketching:

   .. code-block:: yaml

      slots:
        gateway:
          interface: tunnel
          endpoint: 22


   This enables connections to the workshop
   at the default SSH port, :samp:`22`.


#. Add an action to upload your public SSH key to the workshop:

   .. code-block:: yaml
      :caption: workshop.yaml

      actions:
        upload-public-key: |
          PUBLIC_KEY="$1"
          if [ -z "${PUBLIC_KEY}" ]; then
            echo 'cannot upload public key: pass the public key as the argument' 1>&2
            exit 1
          fi
          echo "${PUBLIC_KEY}" >> $HOME/.ssh/authorized_keys


   This appends your public SSH key to the list of authorized keys
   for the :samp:`workshop` user.


#. Refresh the workshop to apply the changes if you haven't done so already:

   .. code-block:: console

      $ workshop refresh


#. Use the action to upload your public SSH key to the workshop, for example:

   .. code-block:: console

      $ workshop run dev upload-public-key "$(cat ~/.ssh/id_rsa.pub)"


The workshop is now ready to accept SSH connections.


Connect with Gateway
--------------------

#. Open JetBrains Gateway.

#. Create a new SSH connection using these values:

   - :guilabel:`Username`: :samp:`workshop`

   - :guilabel:`Host`: :samp:`localhost`

   - :guilabel:`Port`: :samp:`2200`

   - :guilabel:`Specify private key`:
     Your *private* SSH key counterpart to the public key you uploaded earlier.


#. Click :guilabel:`Check connection and continue`.

#. At the next screen,
   select your preferred JetBrains :guilabel:`IDE version`, e.g. PyCharm.

#. Under :guilabel:`Installation options`,
   choose :guilabel:`Customize installation path`
   and set it to a path under :file:`/project/`, e.g. :file:`/project/pycharm/`;
   this ensures the IDE has enough disk space.

#. For :guilabel:`Project directory`, choose :file:`/project/`.

#. Click :guilabel:`Start IDE and connect`,
   then wait for the IDE to install and launch;
   this may take a few minutes.
   After the IDE starts,
   log in and proceed as usual.


Your JetBrains IDE will now run remotely in the workshop
while providing a native desktop experience.


See also
--------

Explanation:

- :ref:`exp_sketch_sdk`
- :ref:`exp_system_sdk`
- :ref:`exp_tunnel_plug`
- :ref:`exp_tunnel_slot`
- :ref:`exp_workshop_definition`


Reference:

- :ref:`ref_tunnel_interface`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_run`
- :ref:`ref_workshop_sketch-sdk`
