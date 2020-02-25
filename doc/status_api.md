# Status API Endpoint & Payload

GET `/v2/event/{id}/status`

No auth needed.

```javascript
{
	"config": {
		// valid types: create, update, destroy
		"type": "create",
		// valid statuses: pending, done, failed
		"status": "pending",
		// Environment name that is being created/updated/destroyed
		"env_name": "elegant-passenger",
		// Pull Request associated with this event
		"repo": "foo/bar",
		"pr": 99,
		// GitHub user that triggered the event
		"github_user": "john.doe",
		// branch and commit SHA for the triggering repo
		"branch": "feature-foo",
		"revision": "asdf1234"
		// config processing duration (time to fetch, parse and validate all acyl.ymls and build charts)
		"processing_time": "100ms",
		"started": "0001-01-01T00:00:00Z",
		"completed": null,
		// map of name to ref (branch)
		"ref_map": {
			"foo/bar": "asdf"
		},
	},
	// tree is a representation of the entire environment, including the triggering repo and all dependencies (inc transitive)
	"tree": {
		"foo/bar": {
			// the triggering repo will always have an empty parent
			"parent": "",
			// image is status information about the image build for this tree node (if any)
			"image": {
				// name is the image Docker repo name
				"name": "quay.io/foo/bar",
				"error": false,
				// a started timestamp but null completed timestamp indicates the image build is running
				"started": "0001-01-01T00:00:00Z",
				"completed": null
			},
			"chart": {
				// if the image field is set, then the chart install must wait for the image build to complete
				// valid statuses: waiting, installing, upgrading, done, failed
				"status": "waiting",
				"started": null,
				"completed": null
			}
		},
		"redis": {
			// every node other than the triggering repo will have a parent set
			"parent": "foo/bar",
			// an empty image field indicates that no image build is necessary for this node
			"image": null,
			"chart": {
				// if no image build is needed, the chart can be installed immediately when all its dependents are available
				"status": "installing",
				"started": "0001-01-01T00:00:00Z",
				"completed": null
			}
		},
		"postgres": {
			"parent": "foo/bar",
			"image": null,
			"chart": {
				// a completed, successful chart install will have both started and completed timestamps set with status set to "done"
				"status": "done",
				"started": "0001-01-01T00:00:00Z",
				"completed": "0001-01-01T00:00:00Z"
			}
		}
	}
}
```