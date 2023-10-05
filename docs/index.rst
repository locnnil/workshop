:relatedlinks: [LXD](https://documentation.ubuntu.com/lxd/en/latest/), [Snap](https://snapcraft.io/docs)

.. _home:

|project|
=========

**|project| is a tool that automates intricate prerequisite setup
for your projects**.

**Define your dev environment in straightforward YAML**.
The tool consumes the definition to create a contained workspace,
installs the dependencies it lists as a number of SDKs,
and attaches their life cycle hooks for run-time control.
IDEs such as Visual Studio Code or JupyterLab
can discover workspaces and use them in their operation,
tidying up your system and streamlining your work.

**Untangle the know-how that was weaved into your project**.
An environment that could take hours of setup
can be launched with one command;
workspaces enhance issue reproduction across platforms,
facilitate collaboration in code reviews,
and confine hackish experiments in lightweight containers.

**Mitigate your setup's complexity with |project|.**
AI/ML, robotics, IoT, EdTech, and similar domains
commonly have less-than-trivial project layouts
that depend on multiple Linux distributions,
a plethora of SDKs from different publishers,
and a grocery list of libraries and programming languages.

---------


In this documentation
---------------------

.. grid:: 1 1 2 2

   .. grid-item:: :doc:`Tutorial <tutorial/index>`

      **Starter instructions** for new users of |project|


   .. grid-item:: :doc:`How-to guides <howto/index>`

      **Step-by-step guides** covering common tasks


   .. grid-item:: :doc:`Explanation <explanation/index>`

      **Discussion and clarification** of key topics


   .. grid-item:: :doc:`Reference <reference/index>`

      **Technical details**, specifications, APIs

---------


Project and community
---------------------

|project| is an emergent project
within the Enterprise Engineering department here at Canonical.

Come and talk to us if you have a project in AI/ML, robotics or any other area
where setting up an environment is a daily or weekly activity
that can easily shave hours off your schedule.
Share with us the frustrating parts of your experience,
and we'll see what |project| can do.
Let us know if you have an SDK or a framework you’d like to try with |project|:
we'll help you get it out there.


- `Code of conduct <https://ubuntu.com/community/ethos/code-of-conduct>`__

- `Pulse reviews on Discourse <https://discourse.canonical.com/c/engineering/sdk/34>`__

- `Mattermost channel <https://chat.canonical.com/canonical/channels/sdk>`__

- `Product map <https://warthogs.atlassian.net/projects/SDK?selectedItem=com.atlassian.plugins.atlassian-connect-plugin:com.herocoders.plugins.jira.epicsmap__epics-map-page>`__

- `Contribution and participation <https://github.com/canonical/workspace/pulls>`__

- `Product and documentation feedback <https://github.com/canonical/workspace/issues>`__

.. toctree::
   :hidden:
   :maxdepth: 2

   tutorial/index
   howto/index
   explanation/index
   reference/index
