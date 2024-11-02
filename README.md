# esb2ha

Download electricity usage data from ESB and (optionally) upload it
to Home Assistant.

**WARNING**: this is a work in progress. It seems working as expected
             but probability of bugs is high and probability of
             documentation is a little low.

If you live in Ireland you likely have electricity provided by an ESB
smart meter.

If this is the case, you can access your electricity usage registering
following
[this](https://www.esbnetworks.ie/existing-connections/meters-and-readings/my-smart-data)
instructions.

While checking the graph on their website is useful and fun, if you
want to get some better insights about your electricity consumption
you need to get the data offline for further analysis.

Despite you can download a nice CSV from their website, logging in
daily to get the latest results is not really my hobby.

Here there is a small command line tool which can automate this for
you. You simply have to register to https://myaccount.esbnetworks.ie,
link your smart meter to it, and you can simply run this tool to
download your data as frequently as you want.

If you are a Home Assistant user you can also upload this data to your
instance and have it ready in the Energy dashboard.

Check the `documentation` directory for a longer explanation.

Please note that the data they provide is not in realtime. So there is
no benefit in running this tool too frequently.

## Disclaimer

ESB is a registered company in Ireland. I'm not affiliated with them
nor they endorsed this tool.

Use this tool at your own risk.

## Future improvements

I should test usability a little better: the login process is
cumbersome and heavily relies on JavaScript, this tool doesn't include
a full browser nor a JavaScript engine. I don't have enough mileage
yet to know if error reporting is good enough and how resilient is
during platform updates.

### Write a proper Home Assistant integration

This is the longest shot for me, I don't know enough Python to do it.

Since this script can be likely put in a crontab and forget about it
there is no big value in a proper integration other than a fancy UI.

But everyone likes a fancy UI...

If you are a Python developer interested in porting this project
please let me know.

NOTE: there is already a python library to download ESB data
https://github.com/badger707/esb-smart-meter-reading-automation