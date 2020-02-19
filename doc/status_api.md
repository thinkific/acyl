# Status API Endpoint & Payload

```javascript
{
	"config": {
		"status": "Creating New Environment",  // "Creating New Environment" || "Updating Environment (teardown)" || "Updating Environment (upgrade in place)" || "Destoying Environment"
		"processing_time": 929292,
		"started": 39393,
		"completed": null,
		"ref_map": {
			"foo/bar": "asdf"
		},
	},
	"tree": {
		"foo/bar": {
			"parent": null,
			"image": {
				"name": "quay.io/foo/bar",
				"error": false,
				"started": 202929, // timestamp
				"completed": null
			},
			"chart": {
				"status": "WAITING", // WAITING || READY || INSTALLING || DONE
				"started": 0, // timestamp
				"completed": null
			}
		},
		"redis": {
			"parent": "foo/bar",
			"image": {
				"name": "",
				"error": false,
				"started": null,
				"completed": null
			},
			"chart": {
				"status": "INSTALLING", // WAITING || READY || INSTALLING || DONE || FAILED
				"started": 202020, // timestamp
				"completed": null
			}
		},
		"postgres": {
			"parent": "foo/bar",
			"image": {
				"name": "",
				"error": false,
				"started": null,
				"completed": null
			},
			"chart": {
				"status": "DONE", // WAITING || READY || INSTALLING || DONE || FAILED
				"started": 202020, // timestamp
				"completed": 299494 // timestamp
			}
		}
	}
}
```