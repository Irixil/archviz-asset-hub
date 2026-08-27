# Account settings

Manage your personal profile, email, password, and linked sign-in methods from **Settings → Account**.

## Profile

![User profile form](/docs/screenshot_user_profile.png)

Your **display name** is visible to workspace members in comments and activity logs. To change it, edit the field and click **Save**.

Your **avatar** appears next to your name throughout the app. To upload one:

1. Click **Upload avatar** and pick an image file (max 5 MB, any common image format)
2. The new photo takes effect immediately

To remove a manually uploaded avatar, click **Remove** below the upload button. If your avatar comes from a linked Google or SSO account, removal is not available here, unlink the provider instead.

## Email address

Your email address is used for login and notifications. To change it:

1. Enter your new address in the **New email** field and click **Send confirmation**
2. A confirmation link is sent to the new address
3. Click the link in that email to complete the change

While a change is pending, a notice shows the address awaiting confirmation. Click **Cancel** next to it to abort.

## Password & sign-in methods

The **Authentication** section shows all sign-in methods attached to your account: password, Google, SSO, and Canva. At least one method must remain connected at all times.

### Set or change your password

- If you have no password yet, fill in **New password** and **Confirm new password**, then click **Save password**
- If you already have a password, you must also enter your **Current password**
- Passwords must be at least 8 characters

### Linked providers

Connect or disconnect external sign-in providers:

| Provider | How to connect                                               |
| -------- | ------------------------------------------------------------ |
| Google   | Click **Connect**, you are redirected to Google to authorize |
| SSO      | Click **Connect**, you are redirected to your OIDC provider  |
| Canva    | Click **Connect**, you are redirected to Canva to authorize  |

To disconnect a provider, click **Disconnect** next to it. This is only allowed when another sign-in method remains connected.

## Delete account

The **Danger zone** section lets you permanently delete your account. This cannot be undone, all your content remains in the workspace but your user record and login credentials are erased.

1. Click **Delete account**
2. If your account has a password, confirm it in the dialog
3. Click **Delete permanently**

You are logged out immediately and redirected to the login page.
