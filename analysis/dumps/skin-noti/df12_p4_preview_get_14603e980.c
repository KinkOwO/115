__int64 sub_14603E980()
{
  __int64 result; // rax
  __int64 v1; // rdi
  _QWORD *v2; // rax
  __int64 ***v3; // rcx
  __int64 **v4; // rsi
  __int64 *v5; // rbx

  result = qword_14E669C18;
  if ( !qword_14E669C18 )
  {
    v1 = sub_146E8BA20(352);
    if ( v1 )
    {
      *(_QWORD *)v1 = off_14A9B4008;
      *(_QWORD *)(v1 + 16) = 0;
      *(_QWORD *)(v1 + 24) = 0;
      v2 = (_QWORD *)sub_146E8BA20(40);
      *v2 = v2;
      v2[1] = v2;
      *(_QWORD *)(v1 + 16) = v2;
      sub_14576E6A0(v1 + 32);
      *(_BYTE *)(v1 + 315) = 1;
      v3 = *(__int64 ****)(v1 + 16);
      *v3[1] = 0;
      v4 = *v3;
      if ( *v3 )
      {
        do
        {
          v5 = *v4;
          ((void (__fastcall *)(__int64 *, _QWORD))*v4[2])((__int64 *)v4 + 2, 0);
          sub_146E9F3A0(v4, 40);
          v4 = (__int64 **)v5;
        }
        while ( v5 );
      }
      **(_QWORD **)(v1 + 16) = *(_QWORD *)(v1 + 16);
      *(_QWORD *)(*(_QWORD *)(v1 + 16) + 8LL) = *(_QWORD *)(v1 + 16);
      *(_QWORD *)(v1 + 24) = 0;
    }
    else
    {
      v1 = 0;
    }
    qword_14E669C18 = v1;
    (**(void (__fastcall ***)(__int64))v1)(v1);
    return qword_14E669C18;
  }
  return result;
}
