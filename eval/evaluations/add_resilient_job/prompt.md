Add durable `reports:generate` background work for user reports.

The payload must carry only the user ID. When the job runs, reload current user state through the existing repository and write the report to the same deterministic user-specific storage path each time, so a retry cannot create duplicate reports. Configure an explicit retry budget and per-attempt timeout when dispatching the job. Keep queue policy in the job boundary, business behavior in the report service, and preserve caller cancellation throughout.

Connect the existing user-created notification flow to this job, keep registration working, add focused coverage, and leave the Project buildable.
