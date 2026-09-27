__int64 sub_145EFAFB0()
{
  __int64 v0; // rcx
  __int64 v1; // rdx
  __int64 v2; // r8
  __int64 v3; // rax
  __int64 result; // rax
  __int64 v5; // rcx

  v0 = qword_14EF2CA88;
  v1 = qword_14EF2CA90;
  if ( qword_14EF2CA88 )
  {
    if ( *(_DWORD *)(qword_14EF2CA88 + 8) )
    {
      v2 = qword_14EF2CA90;
      goto LABEL_6;
    }
  }
  else
  {
    v0 = 0;
  }
  v2 = 0;
LABEL_6:
  v3 = v2 - 48;
  if ( !v2 )
    v3 = 0;
  if ( v3 )
  {
    if ( !v0 || !*(_DWORD *)(v0 + 8) )
      v1 = 0;
    result = v1 - 48;
    if ( !v1 )
      return 0;
  }
  else
  {
    if ( !qword_14EF2CA70 || (v5 = qword_14EF2CA78, !*(_DWORD *)(qword_14EF2CA70 + 8)) )
      v5 = 0;
    result = v5 - 48;
    if ( !v5 )
      return 0;
  }
  return result;
}
