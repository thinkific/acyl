# UI API

The web UI uses several new API endpoints that are gated by cookie session rather than API keys.

These endpoints are intended to be used by the UI frontend and have some differences from the programmatic endpoints mostly for Javascript ergonomics.

## User Envs (GET)

Get all environments for a specific authenticated GitHub user

Response (array of objects):
```json
[
  {
    "repo": "acme/microservice",
    "pull_request": 89,
    "env_name": "some-name",
    "last_event": "2020-04-14T21:01:13Z",
    "status": "success" // "success"/"failed"/"pending"/"destroyed"/"unknown"
  }
]
```
	
## User Env Detail (GET)

Get environment detail for a specific environment

Response (object)
```json
{
    "repo": "acme/microservice",
    "pull_request": 89,
    "env_name": "some-name",
    "last_event": "2020-04-14T21:01:13Z",
    "status": "success", // "success"/"failed"/"pending"/"destroyed"/"unknown"
    "github_user": "joe.smith",
    "pr_head_branch": "feature-foo",
    "k8s_namespace": "nitro-9999-some-name", // empty string if unavailable
    "events": [  // array of event summary objects
        {
            "event_id": "432b346f-b379-4f7a-8eb3-7f7dfd770d37",
            "started": "2020-04-14T21:01:13Z",
            "duration": "10.2s",  // null if not completed
            "type": "create", // "create"/"update"/"destroy"/"unknown"/"default"
            "status": "done" // "done"/"pending"/"failed"/"unknown"/"default"
        }
    ]
}
```
