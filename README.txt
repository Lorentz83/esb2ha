# esb_cli

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
you.  You simply have to register to https://myaccount.esbnetworks.ie,
link your smart meter to it, and you can simply run this tool to
download your data as frequently as you want.

Please note that the data they provide is not in realtime. So there is
no benefit in running this tool every minute.

## Disclaimer

ESB is a registered company in Ireland. I'm not affiliated with them
nor they endorsed this tool.

Use this tool at your own risk.

## Future improvements

I should test usability a little better: the login process is
cumbersome and heavily relies on JavaScript, this tool doesn't include
a full browser nor a JavaScript engine.  I don't have enough mileage
yet to know if error reporting is good enough and how resilient is
during platform updates.

### Home Assistant integration

[Home Assistant](https://home-assistant.io/) is an open source home
automation system. It has a nice energy dashboard that I'd like to
use.

My goal is to find a way to feed this data into Home Assistant so I
can benefit of the visualization already implemented there.

Despite there is no good support to import historical data
[ref](https://community.home-assistant.io/t/improved-support-for-long-term-historic-data/379659)
it looks like it is already
[possible](https://community.home-assistant.io/t/import-old-energy-readings-for-use-in-energy-dashboard/341406/9).

If a proper integration is proven too difficult, there is always the
alternative to feed data directly
[into the
database](https://community.home-assistant.io/t/import-old-energy-readings-for-use-in-energy-dashboard/341406).
Which has the downside to require stopping Home Assistant to
prevent data corruption.

I'm fairly sure it is possible to integrate Home Assistant with external binaries.
But please let me know if you are a Python developer who would like to port this 
library natively in Python.
