import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Privacy Policy",
  description: "How Aurea Link collects, uses, stores, and shares information.",
};

export default function PrivacyPolicy() {
  return (
    <article className="policy-page">
      <header className="policy-heading">
        <p className="eyebrow">Privacy</p>
        <h1>Privacy Policy</h1>
        <p>Last updated September 10, 2026</p>
      </header>

      <div className="policy-content">
        <section>
          <h2>About Aurea Link</h2>
          <p>
            Aurea Link is a file-sharing service for temporary transfers, private file storage,
            folders, and collaboration. This policy explains the information Aurea Link handles
            when you visit the service, upload or receive files, or sign in with Google.
          </p>
        </section>

        <section>
          <h2>Information we collect</h2>
          <p>Depending on how you use Aurea Link, we process:</p>
          <ul>
            <li>
              <strong>Google account information.</strong> When you choose Google Sign-In, Google
              and Firebase Authentication provide a unique account identifier, your email address,
              and your display name. Aurea Link does not request access to Google Drive, Gmail,
              contacts, calendars, or other Google account content.
            </li>
            <li>
              <strong>Files and file information.</strong> We store files you upload along with
              filenames, file types, sizes, upload status, timestamps, and the folders or share
              links you create.
            </li>
            <li>
              <strong>Account and collaboration information.</strong> We store your Aurea Link
              display name, folder structure, sharing permissions, invitations, and storage usage.
            </li>
            <li>
              <strong>Service and security information.</strong> Our hosting providers may process
              request details such as IP address, browser or device information, timestamps, and
              error or security logs when you use the service.
            </li>
            <li>
              <strong>Browser storage.</strong> Firebase uses browser storage to maintain your
              signed-in session. Aurea Link uses IndexedDB to remember resumable-upload details;
              the selected file itself is not copied into IndexedDB.
            </li>
          </ul>
        </section>

        <section>
          <h2>How we use information</h2>
          <p>We use this information to:</p>
          <ul>
            <li>authenticate your account and keep it secure;</li>
            <li>upload, store, organize, preview, share, and download files at your request;</li>
            <li>show your identity to people you collaborate with;</li>
            <li>enforce storage, file-size, expiration, and access limits;</li>
            <li>operate, troubleshoot, and protect the service.</li>
          </ul>
          <p>
            Aurea Link does not sell personal information or Google user data, and does not use it
            for targeted advertising.
          </p>
        </section>

        <section>
          <h2>How information is shared</h2>
          <p>Information is shared only as needed to provide the service:</p>
          <ul>
            <li>
              Anyone with an active public share link can access the files and file details made
              available through that link.
            </li>
            <li>
              Folder owners and members can see information needed for collaboration, including
              display names, email addresses, roles, filenames, and uploader attribution.
            </li>
            <li>
              Google Cloud and Firebase process information as service providers for hosting,
              storage, databases, logging, and authentication.
            </li>
            <li>
              Information may be disclosed when required by law or when reasonably necessary to
              prevent fraud, abuse, or harm.
            </li>
          </ul>
        </section>

        <section>
          <h2>Retention and deletion</h2>
          <p>
            Anonymous transfer links stop working 24 hours after creation. The associated objects
            and metadata may remain after expiration while operational cleanup is completed.
            Files saved to an account remain until the user deletes them. Share links can be
            revoked, and files deleted through the account workspace are removed from active
            storage and application records.
          </p>
          <p>
            Some information may remain temporarily in logs, backups, or security records when
            needed to operate and protect the service. You can remove Aurea Link&apos;s access to
            your Google account from your Google Account connections at any time.
          </p>
        </section>

        <section>
          <h2>Security</h2>
          <p>
            Aurea Link uses encrypted network connections, access controls, private database
            networking, and short-lived signed storage links. No online service can guarantee
            absolute security, so only share a link with people you trust.
          </p>
        </section>

        <section>
          <h2>Your choices</h2>
          <p>
            You can use temporary file sharing without an account, sign out at any time, revoke
            active share links, remove folder members, and delete files from your workspace. You
            can also manage or revoke the Google connection through your Google Account.
          </p>
        </section>

        <section>
          <h2>Children</h2>
          <p>
            Aurea Link is not directed to children under 13, and we do not knowingly collect
            personal information from children under 13.
          </p>
        </section>

        <section>
          <h2>Changes to this policy</h2>
          <p>
            We may update this policy when the service or its data practices change. The date at
            the top of this page identifies the latest version.
          </p>
        </section>
      </div>
    </article>
  );
}
