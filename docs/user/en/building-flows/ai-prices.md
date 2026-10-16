---
title: AI model prices and spending limits
description: Where the price of a model comes from, how to set your own, and how the spending and time limits of an AI agent use it.
sources:
  - ADR-0062
  - ADR-0060
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# AI model prices and spending limits

> This page describes specified behavior that is not implemented yet. The releases that deliver the price table, the price screen and the commands remove this note.

Orcon needs the price of a model to show what an AI agent costs and to stop it when it reaches a spending limit. This page explains where those prices come from, how to set your own and what the limits do.

## Prices are estimates
Orcon multiplies the tokens a model call used by the price it has for that model. The result is an estimate. The invoice of the model provider is always the one that counts. Costs are shown in US dollars.

## Where a price comes from
For each model Orcon uses the first price it finds in this order:

1. A price your organization set for that model.
2. A price the person who runs this Orcon installation set for every organization.
3. The prices that come with Orcon.

A price that you or the operator set replaces the whole entry of the model: the prices of input, output and cached tokens all come from the same place, never mixed.

The prices that come with Orcon were read from the public price page of the model provider by a person, who wrote down the page address and the date. The price screen shows both for each model, so you can open the page and check it. Orcon warns you when the oldest of these prices was confirmed more than 120 days ago. Names of models must match exactly: a model name with a date and the same name without a date are different entries.

A model that has no price in any of these places has an unknown price.

## Set your own price
You need the permission to manage AI prices in your organization. People who do not have it can see the prices but not change them.

1. Open **Settings**, then **AI prices** in your organization.
2. Choose **Set price** on the model, or add the model if it is not in the list.
3. Enter the price in dollars per one million tokens, with up to six decimals, for input, output, cache reads and cache writes. Reasoning tokens and a price for very long inputs are optional.
4. Write a note that says why, for example that the price was negotiated. The note is required.
5. Save.

The screen shows who can change prices and the source of each price as text: "Default, confirmed on a date", "Instance" or "Organization" followed by your note. If someone else changed the same price while you had the form open, Orcon tells you and asks you to reload before you save again. To go back to the default, choose **Remove**.

The same can be done from the command line, with the price in dollars per million tokens:

```
orcon ai prices show
orcon ai prices override set <provider> <model> --input <dollars> --output <dollars> --cache-read <dollars> --cache-write <dollars> --note "<reason>"
orcon ai prices override remove <provider> <model>
```

`<provider>` is the name of the model provider as `orcon ai prices show` prints it, `<model>` is the model name exactly as the node uses it, and `<dollars>` is a number with up to six decimals. Add `--instance` to set or remove a price for the whole installation; that needs the instance permission.

A change can take up to 30 seconds to apply in every part of a large installation. Each model call records which price version it used.

The person who runs the installation can turn off organization prices. When that is done, the screen says so, your existing prices are kept but not used, and you cannot add new ones. The operator can also give the installation a price file; when a file is in use, prices for the whole installation can be changed only by editing that file.

## Spending limit of an AI agent
The AI agent node has a spending limit, `maxSpend`, in micro-dollars (one millionth of a dollar).

- Before each model call, Orcon adds the highest cost that call could have to what the agent has already spent. If the total is above the limit, the call does not happen. The highest cost assumes the model writes as much as it is allowed to write, so the last call can be refused even though it would really have cost less.
- If the agent has a spending limit and its model has an unknown price, Orcon cannot count the cost. When you publish the flow you get an error, with the code `MODEL_PRICE`, and the message says which model and where to add a price. If the price disappears after you publish (for example, someone removes an override), the agent stops before it calls the model, with the error `AI_PRICE_UNKNOWN`. You can route that error like any other.
- An AI node without a spending limit and with an unknown price publishes with a warning. It runs, its cost is recorded as unknown and the call is counted as unpriced.

## Time limit of an AI agent
The AI agent node also has a limit of active time, `maxActiveTime`, written as an ISO 8601 duration such as `PT10M`. The default is 10 minutes and the value must be above zero.

- Active time is the time the agent is working. Time spent waiting, for example for a person to approve a tool, does not count.
- Orcon checks the limit before each model call and before it runs tools. If the limit is reached, the agent stops with the error `AI_AGENT_LIMIT_EXCEEDED`.
- If the limit is reached while the model is answering, Orcon cuts the call. That call is recorded as possibly billed, because the provider may charge for it although no answer arrived. A tool that is already running is not cut, because it may have changed something outside Orcon; the limit applies when it finishes.
- If Orcon stops unexpectedly, the time since the last saved step is not counted.
