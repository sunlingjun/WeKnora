export type MemberUserLike = {
  name?: string
  username?: string
  real_name?: string
  cas_real_name?: string
  email?: string
}

function normalize(value?: string): string {
  return (value || '').trim()
}

function emailsEqual(a?: string, b?: string): boolean {
  const left = normalize(a).toLowerCase()
  const right = normalize(b).toLowerCase()
  return left !== '' && left === right
}

export function emailLocalPart(email: string): string {
  const at = email.indexOf('@')
  return at > 0 ? email.slice(0, at) : email
}

/** Title line: real name, else username, else email local-part. Never repeats a full mailbox. */
export function resolveMemberDisplayName(
  user: MemberUserLike | undefined | null,
  unknownLabel: string,
): string {
  if (!user) return unknownLabel
  const real = normalize(user.cas_real_name || user.real_name)
  if (real) return real
  const email = normalize(user.email)
  const username = normalize(user.username || user.name)
  if (username && !emailsEqual(username, email)) return username
  if (email) return emailLocalPart(email)
  if (username) return username.includes('@') ? emailLocalPart(username) : username
  return unknownLabel
}

/** Second line: full email only when it adds information beyond the title. */
export function resolveMemberEmailLine(
  user: MemberUserLike | undefined | null,
  displayName: string,
): string {
  const email = normalize(user?.email)
  if (!email || emailsEqual(email, displayName)) return ''
  return email
}
