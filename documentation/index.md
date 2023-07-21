The assumption in the whole documentation is that you are already
registered to https://www.esbnetworks.ie and you know:

  1. your user name.
  2. your password.
  3. your mprn number.

You can find the mprn number either on the top of your electricity
bill or (provided you linked it already to your account) in your
personal section of esbnetworks.ie.

# I just wan to give it a quick try

You can!

From a Linux machine with Docker installed, just open a shell and
type:

```
cd testing
export esb_password=[...]
export esb_user=[...]
export mprn=[...]
./ test.sh
```

It will create an ephemeral instance of Home Assistant and uploads
your electricity consumption into it.

You can follow the instructions to login and check if you like how it
works, and once you are done you can just remove the two new Docker
images and it is like nothing ever happened!

# Building the command line tool

There are two options:
  1. using the Go toolchain to build a binary
  2. using docker to build an image

## Using the go toolchain

Refer to https://go.dev/doc/install to install the go building
environment (or use the package manager of your linux distribution)
and simply

```
cd scr
go build github.com/lorentz83/esb2ha
```

You'll find an executable `esb2ha` in the same directory.

It is statically linked, so you can move it to your Home Assistant
server if you prefer (assuming it is the same architecture of the
computer where you compiled it, otherwise you need to cross compile).

## Using Docker

This is as easy as

```
docker build --tag=esb2ha:latest .
```

to create an image named `esb2ha:latest` which contains the binary you
need.

You can also create an alias if you like
```
alias esb2ha='docker run --rm esb2ha:latest'
```

Just remember that:

  - you are running in a docker container, so `localhost` has a
    different meaning.
  - every time the documentation mentions you can set environment
    variables, these are not automatically forwarded to the container.

# Configuring Home Assistant

In order to import data in Home Assistant you need to create a sensor
to collect it.

Open your `configuration.yaml` and add the following section:

```
template:
- trigger:
    sensor:
      name: 'ESB electricity usage' # Feel free to change this.
      availability: 'false'
      state: 'none'
      unit_of_measurement: 'kWh'
      state_class: total_increasing
      device_class: energy
      unique_id: 'esb_electricity_usage' # Feel free to change this.
```

I'm not an expert here, please correct me if I'm wrong. But there are
a few caveat to understand.

This sensor will never really get any data. This is why the trigger
section is empty and it also self reports as unavailable.
Therefore its value will be always unknown and you won't see any
history either.

What we do is to overwrite its statistical data, which will be visible
only from the Energy dashboard.
The best you can do is to hide this sensor from your default dashboard
once it is configured.

The `state_class` mentions that the statistics of this sensor will
increase but can be reset to 0 (which happens every time there is a
new import).

Once the yaml edit is done, open Home Assistant's "Developer tools"
and click "Check configuration", to be sure there is no issue,
followed by "Restart".

At this point you can proceed with the import.

Once done, you can go to the "Energy" dashboard and follow the
instruction to add the new sensor.

Remember that ESB doesn't export data fresher than 24 hours, so the
first graph you'll see will be empty. Don't worry, change day or move
to the weekly or monthly view to see your energy consumption.

You can see the comparison between the graph rendered on ESB and Home
Assistant.

![ESB power consumption graph](esb.png)
![Home Assistant Energy dashboard](home-assistant-energy.png)

Keep in mind that while ESB records a value every half an hour, Home
Assistant recors a value every hour. Therefore minor differences are
expected.

Finally, you need to create an authentication token so that the
command line can connect to upload data to this sensor.

It needs to have admin powers. Select your user from the Home
Assistant side bar. Scroll to the bottom until the section "Long-Lived
Access Tokens" and click "Create token".

The name is useful only for you to remember its purpose and revoke it
once you don't need it anymore. You just need the token, be sure to
write it down because you cannot get it anymore (but you can always
create a new one of course).

# Using esb2ha

The command line contains help that should be pretty self explanatory.

Type `esb2ha help` or `esb2ha help [command]` to see the documentation.

I'll put here just a few remarks.

Currently it can work in 3 modes:

  1. only download the data from esbnetworks.ie website.
  2. only upload the CSV file to Home Assistant.
  3. run both the previous commands in a single step.

Remember that on shared computers passing password as flags is not
recommended because any user can see them (just by running `ps aux`
for example).

Therefore each flag can be passed as environment variable too. Flags
have priority, but if empty the environment variable with the same
name is checked too.

# I need help

Feel free to open a bug. Please try to add as many information as
possible, with the exception of passwords :)

# I want to contribute

Both code and ideas are welcome! :D
