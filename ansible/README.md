## theo-agent Deployment

- Requires Ansible 1.2 or newer

This playbook deploys theo-agent, version  0.5.1.   
To use it, first edit the "hosts" inventory file to contain the
hostnames of the machines on which you want theo-agent deployed, and edit the 
group_vars/all file to set any theo-agent configuration parameters you need.

Then run the playbook, like this:

	ansible-playbook -i hosts site.yml