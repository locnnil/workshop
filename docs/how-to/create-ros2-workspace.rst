.. _how_create_ros2_workspace:

How to create a ROS2 workspace
==============================

For a practical example,
let's create a
`ROS2 workspace
<https://docs.ros.org/en/humble/Tutorials/Beginner-Client-Libraries/Creating-A-Workspace/Creating-A-Workspace.html>`_
using the :samp:`ros2` SDK
that we have published in our SDK Store
under the :samp:`latest/edge` channel.

.. note::

   This example assumes that you have already
   :ref:`installed
   <tutorial>` |project_markup|.
   Also, our ROS2 SDK is built from the :samp:`humble` channel,
   so we're using the
   `appropriate steps
   <https://docs.ros.org/en/humble/Tutorials/Beginner-Client-Libraries/Colcon-Tutorial.html>`_;
   adjust the steps for other channels accordingly.


Create the workshop
--------------------

To create a ROS2 workshop,
you can reuse the following definition:

.. code-block:: yaml
   :caption: .workshop.ros2.yaml

   name: ros2-humble
   base: ubuntu@22.04
   sdks:
     ros2:
       channel: latest/edge


Note that the :samp:`base` needs to be the same as the SDK base,
and the channel should set to :samp:`latest/edge`.

Now, you can create the workshop by running:

.. code-block:: console

   $ workshop launch ros2-humble


The workshop is already enabled for ROS2;
you don't need to install anything else to continue.


Build the examples
------------------

To build the examples,
`clone
<https://docs.ros.org/en/humble/Tutorials/Beginner-Client-Libraries/Colcon-Tutorial.html#add-some-sources>`_
the ROS2 examples repo in your project directory:

.. code-block:: console

   $ git clone https://github.com/ros2/examples src/examples -b humble


Mind that |project_markup| auto-mounts the project directory
inside the workshop as :file:`/project/`;
we'll use this location in the upcoming steps.
Now, open a shell into the workshop and go the project directory:

.. code-block:: console

   $ workshop shell ros2-humble
   workshop@ros2-humble-8584e57d$ cd /project/
   workshop@ros2-humble-8584e57d$ ls

     src


Here, you can see the :file:`src/` directory with the cloned examples.
Now
`build
<https://docs.ros.org/en/humble/Tutorials/Beginner-Client-Libraries/Colcon-Tutorial.html#build-the-workspace>`_
the ROS2 workspace:

.. code-block:: console

   workshop@ros2-8584e57d$ colcon build


This builds the examples in the host-mapped :file:`~/colcon/` directory,
which means the build cache will be auto-reused
after the workshop is stopped and then started, or even refreshed:

.. code-block:: console

   workshop@ros2-8584e57d$ ls ~/colcon/

     build  install  log


If you exit the workshop shell,
the same content can be seen in the default content directory
(note the project ID from the shell prompt above, :samp:`8584e57d`):

.. code-block:: console

   $ ls ~/.local/share/workshop/project/8584e57d/content/

     ros2-humble_ros2_colcon-cache.sdk  ros2-humble_ros2_ros-cache.sdk

   $ ls ~/.local/share/workshop/project/8584e57d/content/ros2-humble_ros2_colcon-cache.sdk/

     build  install  log


However,
removing the workshop destroys this content
unless you have remounted it in advance.

The ROS2 workspace is ready;
from here, you can
`proceed
<https://docs.ros.org/en/humble/Tutorials/Beginner-Client-Libraries/Colcon-Tutorial.html#run-tests>`_
with the tests and examples as usual.


Benefits
--------

Let's reiterate the benefits of using |project_markup| in this situation:

- Little to no setup is required to start:
  the SDK automates the installation of all prerequisites
  and hides the complexity of the ROS2 installation.

- Time and resources saved: the workspace is built in a host-mapped directory,
  so the build cache is preserved across workshop restarts and refreshes.

- Less clutter: the workspace is built inside the workshop,
  so you can run multiple channels side by side in different workshops
  (when the respective SDKs eventually become available)
  without interference between them and the host system.



Bonus: SDK composition
----------------------

Our version of the ROS2 SDK is defined as follows:

.. code-block:: yaml
   :caption: sdk.yaml

   name: ros2
   base: ubuntu@22.04
   summary: The strictly necessary ROS 2 development environment for your project.
   license: LGPL-2.1
   description: |
     The ros2 SDK creates the minimum necessary development environment for your ROS 2 project.
     It sets up a bare minimum ROS 2 workspace before installing all of the dependencies
     for the ROS 2 project mounted by workshop.
   
     A developer can thus connect to the workshop and immediately build the project.
   plugs:
     ros-cache:
       interface: content
       target: /home/workshop/.ros
     colcon-cache:
       interface: content
       target: /home/workshop/colcon
     gpu:
       interface: gpu
     ssh-agent:
       interface: ssh-agent


You can see that it defines two content plugs
for :file:`~/.ros/` and :file:`~/colcon/` directories
*inside* the workshop,
as well as a GPU plug and an SSH agent plug;
while that's not strictly necessary for this example,
this enables preserving the ROS2 build cache and settings,
and also provides extra capabilities.


See also
--------

Explanation:

- :ref:`exp_content_interface`
- :ref:`exp_interfaces_plugs_slots`
- :ref:`exp_sdk`


Reference:


- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_remove`
- :ref:`ref_workshop_shell`
- :ref:`ref_workshop_start`
- :ref:`ref_workshop_stop`