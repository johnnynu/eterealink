# Phase 4 Firebase Authentication Setup

This setup enables Google Sign-In in the browser and Firebase ID-token verification in the Go API. The Firebase web configuration is public application metadata; do not add service-account private keys to the repository.

## 1. Create or select the Firebase project

1. Open the [Firebase console](https://console.firebase.google.com/) and add Firebase to the Google Cloud project used by Eterealink, or create a development Firebase project.
2. In **Project settings**, register a Web app.
3. Copy the web app configuration values for the frontend environment.

## 2. Enable Google Sign-In

1. Open **Authentication** in the Firebase console.
2. On **Sign-in method**, enable the Google provider and choose the support email.
3. Add `localhost` and every deployed frontend host to **Authentication > Settings > Authorized domains**. Projects created after April 28, 2025 do not authorize `localhost` by default, so add it manually for local development. Do not authorize `localhost` in the production Firebase project.

## 3. Configure the API

Set the project ID in the environment used by the Go API:

```bash
FIREBASE_PROJECT_ID=your-firebase-project-id
```

When using the repository's Make targets, place this value in the ignored root `.env` file; Make loads it automatically.

The API uses the project ID to validate token audience and issuer through the [Firebase Admin SDK's ID-token verification](https://firebase.google.com/docs/auth/admin/verify-id-tokens). Verifying ID tokens does not require a downloaded service-account key. The API fetches and caches Google's public signing certificates.

## 4. Configure the frontend

Copy `frontend/.env.example` to `frontend/.env.local` and set the values from the registered Firebase Web app:

```bash
NEXT_PUBLIC_FIREBASE_API_KEY=
NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN=
NEXT_PUBLIC_FIREBASE_PROJECT_ID=
NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET=
NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID=
NEXT_PUBLIC_FIREBASE_APP_ID=
```

The sign-in control is enabled when the API key, auth domain, project ID, and app ID are present. Restart the Next.js development server after changing public environment variables.

## 5. Verify the flow

1. Start PostgreSQL, apply migrations, and run the API and frontend as described in the README.
2. Select **Sign in with Google** and finish the account chooser.
3. Confirm that the header shows the verified user's name and email.
4. Confirm that `users` contains one row for the Firebase UID. Signing out and back in must reuse that row rather than creating a duplicate.
5. Confirm that anonymous uploads still work while signed out.

The browser calls `GET /v1/me` through the same-origin API proxy. Missing or invalid bearer tokens receive `401 Unauthorized`; a backend without `FIREBASE_PROJECT_ID` reports `503 authentication_unavailable` only for protected routes.
