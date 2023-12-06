:relatedlinks: [LXD](https://documentation.ubuntu.com/lxd/en/latest/), [Snap](https://snapcraft.io/docs)

.. _home:

Workshop
========

**Workshop is a tool that fractions
development environment prerequisites
into manageable pieces**.

**Define your environment in simple YAML**.
|project| consumes the definition to create a contained workshop,
installs the dependencies as a set of SDKs
and attaches custom actions for run-time control.
IDEs such as Visual Studio Code or JupyterLab
can discover workshops and use them in day-to-day operations,
tidying up your system and streamlining your work.

**Untangle the know-how woven into your project**.
An environment that could take hours to set up
can now be launched with a single command.
Workshop improves cross-platform issue reproduction,
preserves context in discussions or reviews
and confines bold experiments to transparent sandboxes.

**Mitigate the complexity of your setup.**
AI/ML, robotics, IoT, EdTech and similar domains
typically use less-than-trivial project layouts
that depend on multiple Linux distributions or images,
a plethora of SDKs from many vendors
and a grocery list of libraries and languages.
That's where |project| thrives.

---------


In this documentation
---------------------

.. grid:: 1 1 2 2

   .. grid-item:: :doc:`Tutorial <tutorial/index>`

      **Starter instructions** for new users of |project|


   .. grid-item:: :doc:`How-to guides <how-to/index>`

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

- `Contribution and participation <https://github.com/canonical/workshop/pulls>`__

- `Product and documentation feedback <https://github.com/canonical/workshop/issues>`__

.. toctree::
   :hidden:
   :maxdepth: 2

   Home <self>
   tutorial/index
   how-to/index
   explanation/index
   reference/index
