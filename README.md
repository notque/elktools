# Elktools

This repository contains tooling for working with the elkstack.

Current functionality:

- Restore elasticsearch from swift backups

----

## Restore Elasticsearch from swift backup
1. Clone this repo

   ``` git clone https://github.com/notque/elktools.git ```

2. Create a config.env file or modify the provided config.env file
3. cd to the cloned repository

    ``` cd elktools ```

4. Build the docker image 

    ``` docker build -t elktools . ```

5. Run the docker container, pointing to your config file and passing the appropriate arguments.

    ``` docker run --env-file=location_to_file elktools -elasticHost=yourElasticHost -eventType=doc```

Configuration for all Openstack Variables, including the swift containername should be stored in the configuration file passed to the docker run command.  These will be loaded as environment variables in the container and utiliezed by the elktools utility.

This restore process leverages elasticsearch copies stored in swift.  Access for swift is granted through an Openstack seed which creates the swift container and the ec2 credentials used to access the container.  Seed can be found here:  https://github.com/sapcc/helm-charts/blob/master/openstack/hermes/templates/seed.yaml