.. _how_create_ros2_workshop:

How to create a ROS 2 workshop
==============================

For a practical example,
let's create a workshop for
`ROS 2
<https://docs.ros.org/en/humble/Tutorials/Beginner-Client-Libraries/Creating-A-Workspace/Creating-A-Workspace.html>`_
using the :samp:`ros2` SDK,
which we have published in our SDK Store
under the :samp:`latest/edge` channel.

.. note::

   This guide assumes that you already have |project_markup|
   :ref:`installed <tutorial>` and know your way around it.
   Also, our ROS 2 SDK is currently based on the :samp:`humble` distribution,
   so we'll use its
   `tutorials
   <https://docs.ros.org/en/humble/Tutorials/Beginner-Client-Libraries/Colcon-Tutorial.html>`_;
   adapt these steps for other distributions as needed.


Get the sources ready
---------------------

For demonstration,
let's use the ROS 2 examples.
Choose a project directory
and
`clone
<https://docs.ros.org/en/humble/Tutorials/Beginner-Client-Libraries/Colcon-Tutorial.html#add-some-sources>`_
the repo there:

.. code-block:: console

   $ git clone https://github.com/ros2/examples -b humble


Note that we're using the :samp:`humble` branch to match the SDK we're using;
adjust that part if necessary.


Create the workshop
-------------------

To define a workshop enabled for ROS 2,
save this file in the project directory
beside the sources:

.. code-block:: yaml
   :caption: .workshop.ros2-humble.yaml

   name: ros2-humble
   base: ubuntu@22.04
   sdks:
     ros2:
       channel: latest/edge


Here, :samp:`base` must be the same as the SDK base,
and :samp:`channel` should be set to :samp:`latest/edge`.

You project directory should look like this now:

.. code-block:: console

   $ ls -a

     .  ..  examples  .workshop.ros2-humble.yaml


All set, so launch the workshop:

.. code-block:: console

   $ workshop launch ros2-humble


.. note::

   The ROS 2 environment is prepared and
   `sourced <https://docs.ros.org/en/humble/Tutorials/Beginner-Client-Libraries/Creating-A-Workspace/Creating-A-Workspace.html#source-ros-2-environment>`_
   by the SDK when the workshop is launched;
   the SDK traverses the sources in the project directory
   for extra dependencies and installs them as needed.


Build the project
-----------------

|project_markup| mounts the project directory
inside the workshop as :file:`/project/`,
so open a shell and go there:

.. code-block:: console

   $ workshop shell ros2-humble
   workshop@ros2-humble-8584e57d$ cd /project/
   workshop@ros2-humble-8584e57d$ ls

     examples


Here's the :file:`examples/` directory that you've cloned.

The SDK already took care of installing and setting up the ROS environment,
*including* your project's dependencies,
so you can immediately proceed with the
`build
<https://docs.ros.org/en/humble/Tutorials/Beginner-Client-Libraries/Colcon-Tutorial.html#build-the-workspace>`_:

.. code-block:: console

   workshop@ros2-humble-8584e57d$ colcon build


Upon completion,
the build artefacts can be found in the :file:`~/colcon/` directory:

.. code-block:: console

   workshop@ros2-humble-8584e57d$ ls ~/colcon/

     build  install  log


The SDK maps this directory to the host using the content interface,
so the build cache can be persisted and reused
after the workshop is stopped and started again, or even refreshed.

Try this for yourself:

.. code-block:: console

   workshop@ros2-humble-8584e57d$ exit
   $ workshop refresh ros2-humble
   $ workshop shell ros2-humble
   workshop@ros2-humble-8584e57d$ cd /project/
   workshop@ros2-humble-8584e57d$ colcon build


This time, the build should finish much faster,
even though :command:`refresh` rebuilds the workshop from scratch,
pulling any potential SDK updates.

The host-mapped contents of the workshop can actually be seen
in |project_markup|'s default content directory on the host
(use your project ID from the shell prompt; here, it's :samp:`8584e57d`):

.. code-block:: console

   workshop@ros2-humble-8584e57d$ exit
   $ ls ~/.local/share/workshop/project/8584e57d/content/

     ros2-humble_ros2_apt-archives.sdk  ros2-humble_ros2_colcon-cache.sdk  ros2-humble_ros2_ros-cache.sdk

   $ ls ~/.local/share/workshop/project/8584e57d/content/ros2-humble_ros2_colcon-cache.sdk/

     build  install  log


However,
removing the workshop will eventually destroy this content
unless you have previously remounted it to a non-default location.

Our ROS 2 project is now ready,
so you can run the
`tests
<https://docs.ros.org/en/humble/Tutorials/Intermediate/Testing/CLI.html#examine-test-results>`_
from the project directory:

.. code-block:: console

   $ workshop shell ros2-humble
   workshop@ros2-humble-8584e57d$ cd /project/
   workshop@ros2-humble-8584e57d$ colcon test
   workshop@ros2-humble-8584e57d$ colcon test-result --all


Benefits
--------

Let's review the advantages of using |project_markup| with the ROS 2 SDK:

- **Little or no setup is required to get started**:
  The SDK automates the installation of all prerequisites
  and reduces the inherent complexity of a ROS 2 installation.

- **Saved time and resources**:
  The project is built in a host-mapped directory,
  so the build cache is preserved across workshop restarts and refreshes;
  the SDK also handles the mapping.

- **Less clutter**:
  Everything specific to your ROS 2 project is contained,
  so multiple projects can run in separate workshops
  without interfering with each other or the host system.


See also
--------

Explanation:

- :ref:`exp_content_interface`
- :ref:`exp_plugs_slots`
- :ref:`exp_projects`
- :ref:`exp_sdk`
- :ref:`exp_workshop_def`


Reference:

- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_remove`
- :ref:`ref_workshop_shell`
- :ref:`ref_workshop_start`
- :ref:`ref_workshop_stop`