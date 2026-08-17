Add `GET /api/v1/invoices/:id` to this App.

The invoice package already owns the lookup behavior. Keep HTTP concerns in the transport layer, preserve the existing application and repository boundaries, return the invoice as JSON when it exists, and return a JSON `404` response when it does not.

Follow the Project's established GoForj workflow and conventions. Make the change, add focused coverage where it is useful, and verify that the App builds and the route is registered.
