// ebc10caller_sub_1444EC930_0x1444ec930

__int64 __fastcall sub_1444EC930(__int64 a1)
{
  __int64 v2; // rax
  __int64 v3; // rbx
  __int64 v4; // rax
  __int64 result; // rax
  __int64 v6; // rax
  __int64 v7; // rdi
  int v8; // ebx
  int v9; // ebx
  int v10; // ecx
  unsigned int v11; // ebx
  __int64 v12; // rax
  __int64 v13; // rax
  __int64 v14; // rax
  __int64 v15; // r8
  __int64 v16; // rax
  _QWORD *v17; // rdx
  unsigned __int64 v18; // rdx
  __int64 v19; // rcx
  __int64 v20; // rbx
  __int64 v21; // rax
  __int64 v22; // rcx
  unsigned __int64 v23; // rdx
  __int64 v24; // [rsp+20h] [rbp-60h] BYREF
  __int128 v25; // [rsp+28h] [rbp-58h]
  __int64 v26; // [rsp+40h] [rbp-40h]
  _BYTE v27[16]; // [rsp+48h] [rbp-38h] BYREF
  _QWORD v28[2]; // [rsp+58h] [rbp-28h] BYREF
  __int64 v29; // [rsp+68h] [rbp-18h]
  unsigned __int64 v30; // [rsp+70h] [rbp-10h]

  v26 = -2;
  if ( sub_145EFAFB0() )
  {
    v2 = sub_145EFAFB0();
    if ( (unsigned __int8)sub_145CF6B90(v2) )
    {
      v3 = sub_1429DA6A0(qword_14E683C78);
      v4 = sub_14723C170(100007198);
      return sub_145FEF480(v3, v4);
    }
  }
  if ( sub_145EFAFB0() )
  {
    v6 = sub_145EFAFB0();
    if ( (*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)v6 + 8480LL))(v6) )
    {
      v7 = a1 + 1296;
      if ( !(unsigned __int8)sub_146E9FA80(a1 + 1296) )
      {
        v8 = sub_140193D40(a1 + 1296);
        v9 = v8 - sub_146E9F840(v7);
        v10 = 0;
        if ( v9 > 0 )
          v10 = v9;
        v11 = v10 / 1000;
        v12 = sub_14723C170(19840);
        v13 = sub_146E8CF20(v27, v12, v11);
        v14 = sub_14014F430(v13);
        v28[0] = 0;
        v29 = 0;
        v30 = 7;
        v15 = -1;
        do
          ++v15;
        while ( *(_WORD *)(v14 + 2 * v15) );
        sub_14014C8D0(v28, v14);
        sub_146E8C910(v27);
        v16 = sub_1429DA6A0(qword_14E683C78);
        v17 = v28;
        if ( v30 >= 8 )
          v17 = (_QWORD *)v28[0];
        result = sub_145FEF480(v16, v17);
        if ( v30 >= 8 )
        {
          v18 = 2 * v30 + 2;
          v19 = v28[0];
          if ( v18 >= 0x1000 )
          {
            v18 = 2 * v30 + 41;
            v19 = *(_QWORD *)(v28[0] - 8LL);
            if ( (unsigned __int64)(v28[0] - v19 - 8) > 0x1F )
              sub_148AAF304(v19, v18);
          }
          result = sub_146E9F3A0(v19, v18);
        }
        v29 = 0;
        v30 = 7;
        LOWORD(v28[0]) = 0;
        return result;
      }
      sub_1444F0A40(a1);
    }
  }
  sub_1444EBC10(a1, &v24, 3);
  if ( v24 == (_QWORD)v25 )
  {
    v20 = sub_1429DA6A0(qword_14E683C78);
    v21 = sub_14723C170(100002255);
    result = sub_145FEF480(v20, v21);
  }
  else
  {
    result = sub_14668C520(qword_14E683C78, 34, &v24, 0);
  }
  v22 = v24;
  if ( v24 )
  {
    v23 = (*((_QWORD *)&v25 + 1) - v24) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v23 >= 0x1000 )
    {
      v23 += 39LL;
      v22 = *(_QWORD *)(v24 - 8);
      if ( (unsigned __int64)(v24 - v22 - 8) > 0x1F )
        sub_148AAF304(v22, v23);
    }
    result = sub_146E9F3A0(v22, v23);
    v24 = 0;
    v25 = 0;
  }
  return result;
}

