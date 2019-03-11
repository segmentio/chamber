Thanks for thinking about contributing to Chamber!

Chamber is an open-source project run with ❤️ by Segment. We've made it open source in the hope that other folks will find it useful. That said, making open source software takes a lot of work, so we try to keep Chamber focused on its goals. That means first and foremost supporting the use cases we have at Segment, but any other reasonable additions will be accepted with gratitude.

The purpose of these guidelines is all about setting expectations.

# Feature requests (`enhancement` label)

New features should be requested via Issue first, to decide whether it falls within Chamber's scope. *Don't* start with a feature PR without discussion.

Even if it is decided that a feature fits Chamber's goals, that doesn't imply that someone is working on it. The only people who are obliged to work on a feature are the people who intend to use it. An `enhancement` issue without an assignee or a milestone means that nobody intends to work on it. If you're interested in working on it, just say so and we can assign it to you.

An `enhancement` issue with a milestone means we intend to write it, but haven't decided who will do it yet.

`enhancement` issues are subject to our [Staleness Policy](#Staleness Policy). An `enhancement` that's gone stale means that no one's intending to work on it, which implies the feature isn't really that important. If this isn't the case, commenting during the staleness grace period will freshen it; this should almost always be a commitment to implementing it.

# Timeliness

As a user, there's nothing worse than crafting a beautiful PR with extensive tests only to be met with tumbleweeds from a long-abandoned project. We want to assure you that Chamber is maintained and actively worked on, but give you some guidelines on how long you might expect to wait before things get done.

Issues should be triaged within 1 week, where triaging generally means figuring out which type of issue it is and adding labels.

Pull requests (that have had design approval in an issue) should expect responses within 3 days.

If you're finding we aren't abiding by these timelines, feel free to @-mention someone in [CODEOWNERS](.github/CODEOWNERS) to get our attention. If you're the shy type, don't worry that you're bothering us; you're helping us stick the commitments we've made :)

# Staleness

All issues and PRs are subject to staleness after some period of inactivity. An issue/PR going stale indicates there isn't enough interest in getting it resolved. After some grace period, a stale issue/PR will be closed.

An issue/PR being closed doesn't mean that it will never be addressed, just that there currently isn't any intention to do so.

During the grace period, any activity will reset the staleness counter. Generally speaking, this should be a commitment to making progress.

The current staleness policy is defined in [.github/stale.yml](.github/stale.yml).

Stale issues get the `stale` label.

# Labels

- `bug`: behaviour in Chamber that is obviously wrong (but not necessarily obviously solvable).
- `enhancement`: a new feature. Without an assignee, it's looking for someone to take the reins and get it made.
- `help wanted`: no pressing desire to get this addressed. An easy contribution for someone looking to get started contributing.
- `repro hard` (issue): difficult to repro without specific setup. often of third party software. We'll make an effort to help narrow in on the problem, but probably can't guarantee we'll be able to make a definitive judgment on whether it's a real bug.
- `question`: we'll make an effort to answer your question, but won't guarantee we can solve it.

# Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0-beta.3/) to help generate changelogs and do semver releases. We usually do Squash and Merge to PRs, so PR authors are recommended to use the Conventional Commits format in their PR title.

# Anti-contribution

- Obviously, anything that violates our [Code of Conduct](CODE_OF_CONDUCT.md)
- Noisy comments: "me too!" (:thumbsup: instead) or non-constructive complaining
- Feature PRs without a discussion issue: it's important we agree the feature is in-scope before anyone wastes time writing code or reviewing
